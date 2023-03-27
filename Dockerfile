FROM ubuntu:22.04

RUN apt-get update && apt-get install -y sudo psmisc colorized-logs &&\
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends tzdata &&\
    useradd -m -s /bin/bash -p "" ubuntu &&\
    echo "ubuntu ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers &&\
    echo "ansi2txt < /tmp/out" > /tmp/logterm && chmod +x /tmp/logterm &&\
    echo "DEBIAN_FRONTEND=noninteractive" > /etc/environment
