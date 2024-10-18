#!/bin/bash

set -e
set +x
#git config --global url."https://$GHE_TOKEN@github.ibm.com/".insteadOf "https://github.ibm.com/"
set -x
cd /go/src/github.com/IBM/vpc-node-label-updater
CGO_ENABLED=0 go build -a -ldflags '-X main.vendorVersion='"ibmCSIInitContainer-${TAG}"' -extldflags "-static"' -o /go/bin/ibm-csi-init-container ./ibm-csi-init-container/
