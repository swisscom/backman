#!/bin/bash
set -eu
if [ -z "$*" ]; then echo "Usage: dels3.sh ORG SPACE SERVICE_NAME"; fi
#read input params
org=$1
space=$2
service=$3

#change to org/space and create temporary service key
cf t -o $org -s $space
cf csk $service mykey

#parse s3cmd config from service key
service_key=`cf service-key $service mykey|tail -n 8`
access_key=`echo $service_key|jq -r .accessKey`
host_base=`echo $service_key|jq -r .namespaceHost|cut -d "." -f 2-4`
host_bucket=`echo $service_key|jq -r .namespaceHost`
secret_key=`echo $service_key|jq -r .sharedSecret`

#heredoc to create s3cmd in the users homedir
cat << EOF > $HOME/.s3cfg
[default]
access_key = $access_key
host_base = $host_base
host_bucket = $host_bucket
secret_key = $secret_key
EOF

#delete all buckets
s3cmd ls|awk '{print $3}' | while read bucket; do #loop through buckets
#DEBUG echo bucket: $bucket
#DEBUG s3cmd ls $bucket
s3cmd del --recursive $bucket --force #delete bucket content
s3cmd rb $bucket #remove bucket itself
done

#cleanup
cf delete-service-key $service mykey -f #remove service key
cf delete-service $service -f #delete service
