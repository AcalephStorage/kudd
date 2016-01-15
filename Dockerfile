FROM golang:1.5

MAINTAINER Acaleph "admin@acale.ph"

ENV KUBECTL_VERSION 1.1.3

# get kubectl
RUN wget https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    mv kubectl /kubectl && \
    chmod +x /kubectl

ADD ./docker/kubeconfig ~/.kube/config

ADD . /kudd
WORKDIR /kudd

RUN go get github.com/constabulary/gb/...
RUN gb build ./...

EXPOSE 9000
CMD [	]
ENTRYPOINT [ "/kudd/bin/kudd", "-kubectl", "/kubectl" ]
