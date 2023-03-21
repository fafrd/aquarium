FROM ubuntu:20.04

RUN apt-get update && apt-get install sudo &&\
    useradd -m -s /bin/bash -p "" ubuntu &&\
    echo "ubuntu ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers
