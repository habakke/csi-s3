FROM habakke/csi-s3:latest-full
LABEL maintainers="Håvard Bakke <habakke at matrise.net>"
LABEL description="csi-s3 testing image"

RUN apt-get update && \
  apt-get install -y \
  git wget make && \
  rm -rf /var/lib/apt/lists/*

RUN wget -q https://dl.google.com/go/go1.16.2.linux-amd64.tar.gz && \
  tar -xf go1.16.2.linux-amd64.tar.gz && \
  rm go1.16.2.linux-amd64.tar.gz && \
  mv go /usr/local

ENV GOROOT /usr/local/go
ENV GOPATH /go
ENV PATH=$GOPATH/bin:$GOROOT/bin:$PATH

RUN wget -q https://dl.min.io/server/minio/release/linux-amd64/minio && \
  chmod +x minio &&\
  mv minio /usr/local/bin

WORKDIR /app

# prewarm go mod cache
COPY go.mod .
COPY go.sum .
RUN go mod download

ADD test/test.sh /usr/local/bin

ENTRYPOINT ["/usr/local/bin/test.sh"]
