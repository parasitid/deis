:title: Security considerations
:description: Security considerations for Deis.

.. _security_considerations:

Security considerations
========================

.. important::

    Deis is not suitable for multi-tenant environments
    or hosting untrusted code.

A major goal of Deis is to be operationally secure and trusted by operations engineers in every deployed
environment. There are, however, two notable security-related considerations to be aware of
when deploying Deis.


Access to etcd
--------------
Since all Deis configuration settings are stored in etcd (including passwords, keys, etc.), any access
to the etcd cluster compromises the security of the entire Deis installation. The various provision
scripts configure the etcd daemon to only listen on the private network interface, but any host or
container with access to the private network has full access to etcd. This also includes deployed
application containers, which cannot be trusted.

The planned approach is to configure iptables on the machines to prevent unauthorized access from
containers. Some requirements include:

* Containers must be able to access the outside world
* Containers must be able to access other containers
* Containers cannot access the CoreOS host (SSH, etcd, etc)

In practice, this is really only a concern when clusters are running untrusted applications.
Further discussion about this approach is appreciated in GitHub issue `#986`_.

Application runtime segregation
-------------------------------
Users of Deis often want to deploy their applications to separate environments
(commonly: development, staging, and production). Typically, physical network isolation isn't
the goal, but rather segregation of application environments - if a development app goes haywire,
it shouldn't affect production applications that are running in the cluster.

In Deis, deployed applications can be segregated by using the ```deis tags``` command. This
enables you to tag machines in your cluster with arbitrary metadata, then configure your applications
to be scheduled to machines which match the metadata.

For example, if some machines in your cluster are tagged with ```environment=production``` and some
with ```environment=staging```, you can configure an application to be deployed to the production
environment by using ```deis tags set environment=production```. Deis will pass this configuration
along to the scheduler, and your applications in different environments on running on separate
hardware.

.. _deis_on_public_clouds:

Running Deis on Public Clouds
-----------------------------
If you are running on a public cloud without security group features, you will have to set up
security groups yourself through either ``iptables`` or a similar tool. The only ports that should
be exposed to the public are:

 - 22: for remote SSH
 - 80: for the routers
 - 443: (optional) routers w/ SSL enabled
 - 2222: for the builder

For providers that do not supply a security group feature, please try
`contrib/util/custom-firewall.sh`_.

.. _`#986`: https://github.com/deis/deis/issues/986
.. _`contrib/util/custom-firewall.sh`: https://github.com/deis/deis/blob/master/contrib/util/custom-firewall.sh
