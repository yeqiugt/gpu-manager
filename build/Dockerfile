ARG base_img
FROM nvidia/cuda:11.2.0-devel-ubuntu18.04 as build

RUN apt update && apt install -y --no-install-recommends \
  curl

RUN curl -s -L https://nvidia.github.io/libnvidia-container/gpgkey | apt-key add -

RUN apt install -y --no-install-recommends \
  cmake git && \
  rm -rf /var/lib/apt/lists/*

ARG version
ARG commit

ENV GOLANG_VERSION 1.14.3
RUN curl -sSL https://dl.google.com/go/go${GOLANG_VERSION}.linux-arm64.tar.gz \
    | tar -C /usr/local -xz
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

RUN mkdir -p /root/rpmbuild/{SPECS,SOURCES}

COPY gpu-manager.spec /root/rpmbuild/SPECS
COPY gpu-manager-source.tar.gz /root/rpmbuild/SOURCES

RUN echo '%_topdir /root/rpmbuild' > /root/.rpmmacros \
  && echo '%__os_install_post %{nil}' >> /root/.rpmmacros \
  && echo '%debug_package %{nil}' >> /root/.rpmmacros
WORKDIR /root/rpmbuild/SPECS
RUN rpmbuild -bb --quiet \
  --define 'version '${version}'' \
  --define 'commit '${commit}'' \
  gpu-manager.spec

FROM $base_img

ARG version
ARG commit

COPY --from=build /root/rpmbuild/RPMS/x86_64/gpu-manager-${version}-${commit}.el7.x86_64.rpm /tmp

RUN yum install epel-release -y && \
  yum install -y which jq

# Install packages
RUN rpm -ivh /tmp/gpu-manager-${version}-${commit}.el7.x86_64.rpm \
	&& rm -rf /tmp/gpu-manager-${version}-${commit}.el7.x86_64.rpm

# kubelet
VOLUME ["/var/lib/kubelet/device-plugins"]

# gpu manager storage
VOLUME ["/etc/gpu-manager/vm"]
VOLUME ["/etc/gpu-manager/vdriver"]
VOLUME ["/var/log/gpu-manager"]

# nvidia library search location
VOLUME ["/usr/local/host"]

RUN echo "/usr/local/nvidia/lib" > /etc/ld.so.conf.d/nvidia.conf && \
    echo "/usr/local/nvidia/lib64" >> /etc/ld.so.conf.d/nvidia.conf

ENV PATH=$PATH:/usr/local/nvidia/bin

# cgroup
VOLUME ["/sys/fs/cgroup"]

# display
EXPOSE 5678

COPY start.sh /
COPY copy-bin-lib.sh /

CMD ["/start.sh"]
