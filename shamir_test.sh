#!/usr/bin/env bash

#       D<---C<---B<---A
#       |    |
#       \/  \/
#  G--->E--->F<---H
#

killall Peerster

sleep 1

cd ~/go/src/github.com/ksei/Peerster
go build
cd client
go build

for i in `seq 1 8`;
do
	mkdir -p ~/go/src/github.com/test_$i
	cd ~/go/src/github.com/test_$i
	rm -rf ./*
	cp -R ~/go/src/github.com/ksei/Peerster ~/go/src/github.com/test_$i/
done


cd ~/go/src/github.com/test_1/Peerster
./Peerster -UIPort=8081 -gossipAddr=127.0.0.1:5001 -name=A -peers=127.0.0.1:5002 -rtimer 1 -N 8 > ~/Documents/A.out &
cd ~/go/src/github.com/test_2/Peerster
./Peerster -UIPort=8082 -gossipAddr=127.0.0.1:5002 -name=B -peers=127.0.0.1:5003 -rtimer 1 -N 8 > ~/Documents/B.out &
cd ~/go/src/github.com/test_3/Peerster
./Peerster -UIPort=8083 -gossipAddr=127.0.0.1:5003 -name=C -peers=127.0.0.1:5004,127.0.0.1:5006 -rtimer 1 -N 8 > ~/Documents/C.out &
cd ~/go/src/github.com/test_4/Peerster
./Peerster -UIPort=8084 -gossipAddr=127.0.0.1:5004 -name=D -peers=127.0.0.1:5005 -rtimer 1 -N 8 > ~/Documents/D.out &
cd ~/go/src/github.com/test_5/Peerster
./Peerster -UIPort=8085 -gossipAddr=127.0.0.1:5005 -name=E -peers=127.0.0.1:5006 -rtimer 1 -N 8 > ~/Documents/E.out &
cd ~/go/src/github.com/test_6/Peerster
./Peerster -UIPort=8086 -gossipAddr=127.0.0.1:5006 -name=F -peers="" -rtimer 1 -N 8 > ~/Documents/F.out &
cd ~/go/src/github.com/test_7/Peerster
./Peerster -UIPort=8087 -gossipAddr=127.0.0.1:5007 -name=G -peers=127.0.0.1:5005 -rtimer 1 -N 8 > ~/Documents/G.out &
cd ~/go/src/github.com/test_8/Peerster
./Peerster -UIPort=8088 -gossipAddr=127.0.0.1:5008 -name=H -peers=127.0.0.1:5006 -rtimer 1 -N 8 > ~/Documents/H.out &

sleep 5

cd ~/go/src/github.com/test_7/Peerster/client
./client -UIPort="8087" -masterKey="liug" -accountName="twitter" -username="tester" -password="mnbvcx"
echo "test_7 add :masterKey=liug , accountName=twitter , username=tester , password=mnbvcx"

cd ~/go/src/github.com/test_6/Peerster/client
./client -UIPort="8086" -masterKey="hfds" -accountName="facebook" -username="tester" -password="tzhncvb"
echo "test_6 add :masterKey=hfds , accountName=facebook , username=tester , password=tzhncvb"

sleep 2

cd ~/go/src/github.com/test_7/Peerster/client
./client -UIPort="8087" -masterKey="liug" -accountName="twitter" -username="tester"
echo "test_7  :masterKey=liug , accountName=twitter , username=tester , retreive"

cd ~/go/src/github.com/test_5/Peerster/client
./client -UIPort="8085" -masterKey="liug" -accountName="twitter" -username="tester"
echo "test_5  :masterKey=liug , accountName=twitter , username=tester , retreive (should get badcredential)"
