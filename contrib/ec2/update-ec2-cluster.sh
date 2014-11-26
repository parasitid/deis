#!/usr/bin/env bash
#
# Usage: ./update-ec2-cluster.sh [name]
# The [name] is the CloudFormation stack name, and defaults to 'deis'

if [ -z "$1" ]
  then
    NAME=deis
  else
    NAME=$1
fi

set -e

THIS_DIR=$(cd $(dirname $0); pwd) # absolute path
CONTRIB_DIR=$(dirname $THIS_DIR)

source $CONTRIB_DIR/utils.sh

# check for EC2 API tools in $PATH
if ! which aws > /dev/null; then
  echo_red 'Please install the AWS command-line tool and ensure it is in your $PATH.'
  exit 1
fi

# check that the CoreOS user-data file is valid
$CONTRIB_DIR/util/check-user-data.sh

# update the deis EC2 cloudformation
aws cloudformation update-stack \
    --template-body "$(./gen-json.py)" \
    --stack-name $NAME \
    --parameters "$(<cloudformation.json)"

echo_green "Your Deis cluster CloudFormation has been successfully updated."
