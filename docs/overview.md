# Overview on how p2p traffic forwarding works
## Peer discovery
In order to establish the connectivity between two nodes, the first thing we need to do is
actually discover the peer to connect to. Traditional solutions like TURN/STUN/ICE does not
address this. Below are the commonly-used techniques to discover peers.

### mDNS
This is how airplay/DLNA etc discover your other devices. Unfortunately, mDNS relies on multicast to work, which
means normally it works only on your LAN (not always, for example, mDNS zerotier works over WAN by
creating virtual network devices).

### DHT
This is how bittorrent etc discover which nodes to fetch the data. One downside of DHT is that, unlike mDNS,
it requires bootstrapping nodes. It is normally expected that the network nodes have publicly accessible network
address.

### gossip
This is how nodes in bitcoin network etc broadcast out the new mined block. One of the greatest advantage of gossip
protocols are that participants do not need public IPs. What they need to do is to keep connection to nodes that do
have public IPs. Gossip protocols are susceptible to the same weakness of DHT, i.e. requiring bootstrapping and
are quite verbose.

## Connection Establishment
Having learned the identity and addresses information of the peer to connect, we can
now try to establish connection between peers. Depending on the concrete network situation,
there may be a few possibilities.

### Active port mapping by UPNP-IGD/NAT-PMP
In case when the peer do have a public IP and the gateway router supports active port mapping standards
like UPNP-IGD or NAT-PMP, all we have to do is ask the route to actively forward all traffic to some
ports on its WAN interface to our program.

### Hole punching
If we are not so fortunate to have such a luxury, we then need the coorperation from the peer to punch 
a hole in the NAT. In order to do such thing, we need to first notify the peer our intention. That is
why a both connected-to signalling server is needed. Libp2p makes this signalling process more decentralized
by first everyone can be a signalling server and second the association of one peer to the signalling server
can be learned decentralizedly (by DHT or gossip like process).

### Relay
As a last resort, we can relay traffic through an inter-mediate server. This is not scalable.

## Traffic forwarding
After we have create a stream (basically a normal network connection, but multiple logic stream
can be multiplexed on a single network connection), we can then forward traffic between local
and remote socket. This is the least intriguing part. We only want to emphasize that a signle
established connection can be multiplexed to several streams. This way, we can create many forwarding
pipes as we wish.

