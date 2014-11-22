#!/bin/bash
apt-get update
apt-get install -y 


sudo apt-get update
sudo apt-get install git-core
git config --global user.name "James Synge"
git config --global user.email "james.synge@gmail.com"
wget https://storage.googleapis.com/golang/go1.3.3.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.3.3.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
sudo bash
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
mkdir $HOME/go
echo 'export GOPATH=$HOME/go' >> .bashrc
echo 'export PATH=$PATH:$GOPATH/bin' >> .bashrc
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

cat <<EOF > /var/www/index.html
<html><body><h1>Hello World</h1>
<p>This page was created from a simple startup script!</p>
</body></html>
EOF