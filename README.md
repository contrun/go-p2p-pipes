# [WIP] A truly decentralized port-forwarding program built on top of libp2p

## Why?
There are a ton of tunneling software and services off the shelf. Some proprietary software/services
are quite easy to use. And there are a lot of [tunnelling solutions](https://github.com/anderspitman/awesome-tunneling)
that are self-hostable. But most of they only have a subset of the following features.

- Mesh network instead of star topology, i.e. everyone can connect to everyone
- Using LAN connection when possible
- NAT travesal when possible, relay as the last resort
- Unix domain socket forwarding (because the total TCP/UDP port number is limited)
- No centralized coordinating server
- Self organizing (do not need to do extraneous configuration)
- Open and and self-hostable
- Flexible (can forward traffic on demand instead pre-allocating ports, does not need to use a VPN)

## How?
TL;DR libp2p is magic. We relies on libp2p to discover peers and establish connection between peers.
After that we create a libp2p stream to forward our local traffic to remote listening socket.

### [High level overview of how this works](./docs/overview.md)

### [Tutorial on using the tools in this repo](./docs/tutorial.md)

## What's working, what's not?
### Known to work
- Forwarding local traffic to remote socket

### Works partially
- Discovering peers via DHT (not reliably)

### Planned
- Discovering peers via mDNS
- Maintaining peer information via pubsub
- Broadcasting and connecting to relay addresses
- Choosing only to use to hole punched connection

### Currently out of range
- Multiple path
- Udp traffic forwarding

# Similar projects
- [mudler/edgevpn: :sailboat: The immutable, decentralized, statically built p2p VPN without any central server and automatic discovery! Create decentralized introspectable tunnels over p2p with shared tokens](https://github.com/mudler/edgevpn)
- [libp2p/go-libp2p-daemon: a libp2p-backed daemon wrapping the functionalities of go-libp2p for use in other languages](https://github.com/libp2p/go-libp2p-daemon)
