FROM ubuntu:22.04

RUN apt-get update && apt-get install sudo psmisc ansifilter &&\
    useradd -m -s /bin/bash -p "" ubuntu &&\
    echo "ubuntu ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers
