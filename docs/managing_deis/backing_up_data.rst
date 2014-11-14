:title: Backing Up and Restoring Data
:description: Backing up stateful data on Deis.

.. _backing_up_data:

Backing Up and Restoring Data
=============================

While applications deployed on Deis follow the Twelve-Factor methodology and are thus stateless,
Deis maintains platform state in the :ref:`Store` component.

The store component runs `Ceph`_, and is used by the :ref:`Database`, :ref:`Registry`,
:ref:`Controller`, and :ref:`Logger` components as a data store. Database and registry
use store-gateway and controller and logger use store-volume. Being backed by the store component
enables these components to move freely around the cluster while their state is backed by store.

The store component is configured to still operate in a degraded state, and will automatically
recover should a host fail and then rejoin the cluster. Total data loss of Ceph is only possible
if all of the store containers are removed. However, backup of Ceph is fairly straightforward, and
is recommended before :ref:`Upgrading Deis <upgrading-deis>`.

Data stored in Ceph is accessible in two places: on the CoreOS filesystem at ``/var/lib/deis/store``
and in the store-gateway component. Backing up this data is straightforward - we can simply tarball
the filesystem data, and use any S3-compatible blob store tool to download all files in the
store-gateway component.

Setup
-----

The ``deis-store-gateway`` component exposes an S3-compatible API, so we can use a tool like `s3cmd`_
to work with the object store. First, install our fork of s3cmd with a patch for Ceph support:

.. code-block:: console

    $ pip install git+https://github.com/deis/s3cmd

We'll need the generated access key and secret key for use with the gateway. We can get these using
``deisctl``, either on one of the cluster machines or on a remote machine with ``DEISCTL_TUNNEL`` set:

.. code-block:: console

    $ deisctl config store get gateway/accessKey
    $ deisctl config store get gateway/secretKey

Back on the local machine, run ``s3cmd --configure`` and enter your access key and secret key.
Other settings can be left at the defaults. If the configure script prompts you to test the credentials,
skip that step - it will try to authenticate against Amazon S3 and fail.

You'll need to change a few additional configuration settings. First, edit ``~/.s3cfg`` and change
``host_base`` and ``host_bucket`` to match ``deis-store.<your domain>``. For example, for my local
Vagrant setup, I've changed the lines to:

.. code-block:: console

    host_base = deis-store.local3.deisapp.com
    host_bucket = deis-store.local3.deisapp.com/%(bucket)

You'll also need to enable ``use_path_mode``:

.. code-block:: console

    use_path_mode = True

We can now use ``s3cmd`` to back up and restore data from the store-gateway.

Backing up
----------

Database backups and registry data
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

The store-gateway component stores database backups and is used to store data for the registry.
On our local machine, we can use ``s3cmd sync`` to copy the objects locally:

.. code-block:: console

    $ s3cmd sync s3://db_wal .
    $ s3cmd sync s3://registry .

Log data
~~~~~~~~

The store-volume service mounts a filesystem which is used by the controller and logger components
to store and retrieve application and component logs.

Since this is just a POSIX filesystem, you can simply tarball the contents of this directory
and rsync it to a local machine:

.. code-block:: console

    $ ssh core@<hostname> 'cd /var/lib/deis/store && sudo tar cpzf ~/store_file_backup.tar.gz .'
    tar: /var/lib/deis/store/logs/deis-registry.log: file changed as we read it
    $ rsync -avhe ssh core@<hostname>:~/store_file_backup.tar.gz .

Note that you'll need to specify the SSH port when using Vagrant:

.. code-block:: console

    $ rsync -avhe 'ssh -p 2222' core@127.0.0.1:~/store_file_backup.tar.gz .

Note the warning - in a running cluster the log files are constantly being written to, so we are
preserving a specific moment in time.

Database data
~~~~~~~~~~~~~

While backing up the Ceph data is sufficient (as database ships backups and WAL logs to store),
we can also back up the PostgreSQL data using ``pg_dumpall`` so we have a text dump of the database.

We can identify the machine running database with ``deisctl list``, and from that machine:

.. code-block:: console

    core@deis-1 ~ $ docker exec deis-database sudo -u postgres pg_dumpall > dump_all.sql
    core@deis-1 ~ $ docker cp deis-database:/app/dump_all.sql .

Restoring
---------

.. note::

    Restoring data is only necessary when deploying a new cluster. Most users will use the normal
    in-place upgrade workflow which does not require a restore.

We want to restore the data on a new cluster before the rest of the Deis components come up and
initialize. So, we will install the whole platform, but only start the store components:

.. code-block:: console

    $ deisctl install platform
    $ deisctl start store-monitor
    $ deisctl start store-daemon
    $ deisctl start store-metadata
    $ deisctl start store-gateway
    $ deisctl start store-volume

We'll also need to start a router so we can access the gateway:

.. code-block:: console

    $ deisctl start router@1

The default maximum body size on the router is too small to support large uploads to the gateway,
so we need to increase it:

.. code-block:: console

    $ deisctl config router set bodySize=100m

The new cluster will have generated a new access key and secret key, so we'll need to get those again:

.. code-block:: console

    $ deisctl config store get gateway/accessKey
    $ deisctl config store get gateway/secretKey

Edit ``~/.s3cfg`` and update the keys.

Now we can restore the data!

Database backups and registry data
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Because neither the database nor registry have started, the bucket we need to restore to will not
yet exist. So, we'll need to create those buckets:

.. code-block:: console

    $ s3cmd mb s3://db_wal
    $ s3cmd mb s3://registry

Now we can restore the data:

.. code-block:: console

    $ s3cmd sync basebackups_005 s3://db_wal
    $ s3cmd sync wal_005 s3://db_wal
    $ s3cmd sync registry s3://registry

Log data
~~~~~~~~

Once we copy the tarball back to one of the CoreOS machines, we can extract it:

.. code-block:: console

    $ rsync -avhe ssh store_file_backup.tar.gz core@<hostname>:~/store_file_backup.tar.gz
    $ ssh core@<hostname> 'cd /var/lib/deis/store && sudo tar -xzpf ~/store_file_backup.tar.gz --same-owner'

Note that you'll need to specify the SSH port when using Vagrant:

.. code-block:: console

    $ rsync -avhe 'ssh -p 2222' store_file_backup.tar.gz core@127.0.0.1:~/store_file_backup.tar.gz

Finishing up
~~~~~~~~~~~~

Now that the data is restored, the rest of the cluster should come up normally with a ``deisctl start platform``.

The last task is to instruct the controller to re-write user keys, application data, and domains to etcd.
Log into the machine which runs deis-controller and run the following. Note that the IP address to
use in the ``export`` command should correspond to the IP of the host machine which runs this container.

.. code-block:: console

    $ nse deis-controller
    $ cd /app
    $ export ETCD=172.17.8.100:4001
    ./manage.py shell <<EOF
    from api.models import *
    [k.save() for k in Key.objects.all()]
    [a.save() for a in App.objects.all()]
    [d.save() for d in Domain.objects.all()]
    EOF
    $ exit

.. note::

  The database keeps track of running application containers. Since this is a fresh cluster, it is
  advisable to ``deis scale <proctype>=0`` and then ``deis scale`` back up to the desired number of
  containers for an application. This ensures the database has an accurate view of the cluster.

That's it! The cluster should be fully restored.

.. _`Ceph`: http://ceph.com
.. _`s3cmd`: http://s3tools.org/
