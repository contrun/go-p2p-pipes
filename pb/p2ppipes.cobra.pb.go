// Code generated by protoc-gen-cobra. DO NOT EDIT.

package pb

import (
	client "github.com/NathanBaulch/protoc-gen-cobra/client"
	flag "github.com/NathanBaulch/protoc-gen-cobra/flag"
	iocodec "github.com/NathanBaulch/protoc-gen-cobra/iocodec"
	cobra "github.com/spf13/cobra"
	grpc "google.golang.org/grpc"
	proto "google.golang.org/protobuf/proto"
)

func P2PPipeClientCommand(options ...client.Option) *cobra.Command {
	cfg := client.NewConfig(options...)
	cmd := &cobra.Command{
		Use:   cfg.CommandNamer("P2PPipe"),
		Short: "P2PPipe service client",
		Long:  "",
	}
	cfg.BindFlags(cmd.PersistentFlags())
	cmd.AddCommand(
		_P2PPipeStartDiscoveringPeersCommand(cfg),
		_P2PPipeStopDiscoveringPeersCommand(cfg),
		_P2PPipeListPeersCommand(cfg),
		_P2PPipeListDiscoveredPeersCommand(cfg),
		_P2PPipeStartForwardingIOCommand(cfg),
		_P2PPipeStopForwardingIOCommand(cfg),
	)
	return cmd
}

func _P2PPipeStartDiscoveringPeersCommand(cfg *client.Config) *cobra.Command {
	req := &StartDiscoveringPeersRequest{}

	cmd := &cobra.Command{
		Use:   cfg.CommandNamer("StartDiscoveringPeers"),
		Short: "StartDiscoveringPeers RPC client",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.UseEnvVars {
				if err := flag.SetFlagsFromEnv(cmd.Parent().PersistentFlags(), true, cfg.EnvVarNamer, cfg.EnvVarPrefix, "P2PPipe"); err != nil {
					return err
				}
				if err := flag.SetFlagsFromEnv(cmd.PersistentFlags(), false, cfg.EnvVarNamer, cfg.EnvVarPrefix, "P2PPipe", "StartDiscoveringPeers"); err != nil {
					return err
				}
			}
			return client.RoundTrip(cmd.Context(), cfg, func(cc grpc.ClientConnInterface, in iocodec.Decoder, out iocodec.Encoder) error {
				cli := NewP2PPipeClient(cc)
				v := &StartDiscoveringPeersRequest{}

				if err := in(v); err != nil {
					return err
				}
				proto.Merge(v, req)

				res, err := cli.StartDiscoveringPeers(cmd.Context(), v)

				if err != nil {
					return err
				}

				return out(res)

			})
		},
	}

	flag.EnumVar(cmd.PersistentFlags(), &req.Method, cfg.FlagNamer("Method"), "")
	_Dht := &DHTDiscoveryArguments{}
	cmd.PersistentFlags().Bool(cfg.FlagNamer("Dht"), false, "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("Dht"), func() { req.Arguments = &StartDiscoveringPeersRequest_Dht{Dht: _Dht} })
	cmd.PersistentFlags().StringVar(&_Dht.Rv, cfg.FlagNamer("Dht Rv"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("Dht Rv"), func() { req.Arguments = &StartDiscoveringPeersRequest_Dht{Dht: _Dht} })

	return cmd
}

func _P2PPipeStopDiscoveringPeersCommand(cfg *client.Config) *cobra.Command {
	req := &StopDiscoveringPeersRequest{}

	cmd := &cobra.Command{
		Use:   cfg.CommandNamer("StopDiscoveringPeers"),
		Short: "StopDiscoveringPeers RPC client",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.UseEnvVars {
				if err := flag.SetFlagsFromEnv(cmd.Parent().PersistentFlags(), true, cfg.EnvVarNamer, cfg.EnvVarPrefix, "P2PPipe"); err != nil {
					return err
				}
				if err := flag.SetFlagsFromEnv(cmd.PersistentFlags(), false, cfg.EnvVarNamer, cfg.EnvVarPrefix, "P2PPipe", "StopDiscoveringPeers"); err != nil {
					return err
				}
			}
			return client.RoundTrip(cmd.Context(), cfg, func(cc grpc.ClientConnInterface, in iocodec.Decoder, out iocodec.Encoder) error {
				cli := NewP2PPipeClient(cc)
				v := &StopDiscoveringPeersRequest{}

				if err := in(v); err != nil {
					return err
				}
				proto.Merge(v, req)

				res, err := cli.StopDiscoveringPeers(cmd.Context(), v)

				if err != nil {
					return err
				}

				return out(res)

			})
		},
	}

	flag.EnumVar(cmd.PersistentFlags(), &req.Method, cfg.FlagNamer("Method"), "")
	_Dht := &DHTDiscoveryArguments{}
	cmd.PersistentFlags().Bool(cfg.FlagNamer("Dht"), false, "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("Dht"), func() { req.Arguments = &StopDiscoveringPeersRequest_Dht{Dht: _Dht} })
	cmd.PersistentFlags().StringVar(&_Dht.Rv, cfg.FlagNamer("Dht Rv"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("Dht Rv"), func() { req.Arguments = &StopDiscoveringPeersRequest_Dht{Dht: _Dht} })

	return cmd
}

func _P2PPipeListPeersCommand(cfg *client.Config) *cobra.Command {
	req := &ListPeersRequest{}

	cmd := &cobra.Command{
		Use:   cfg.CommandNamer("ListPeers"),
		Short: "ListPeers RPC client",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.UseEnvVars {
				if err := flag.SetFlagsFromEnv(cmd.Parent().PersistentFlags(), true, cfg.EnvVarNamer, cfg.EnvVarPrefix, "P2PPipe"); err != nil {
					return err
				}
				if err := flag.SetFlagsFromEnv(cmd.PersistentFlags(), false, cfg.EnvVarNamer, cfg.EnvVarPrefix, "P2PPipe", "ListPeers"); err != nil {
					return err
				}
			}
			return client.RoundTrip(cmd.Context(), cfg, func(cc grpc.ClientConnInterface, in iocodec.Decoder, out iocodec.Encoder) error {
				cli := NewP2PPipeClient(cc)
				v := &ListPeersRequest{}

				if err := in(v); err != nil {
					return err
				}
				proto.Merge(v, req)

				res, err := cli.ListPeers(cmd.Context(), v)

				if err != nil {
					return err
				}

				return out(res)

			})
		},
	}

	flag.EnumVar(cmd.PersistentFlags(), &req.PeerType, cfg.FlagNamer("PeerType"), "")

	return cmd
}

func _P2PPipeListDiscoveredPeersCommand(cfg *client.Config) *cobra.Command {
	req := &ListDiscoveredPeersRequest{}

	cmd := &cobra.Command{
		Use:   cfg.CommandNamer("ListDiscoveredPeers"),
		Short: "ListDiscoveredPeers RPC client",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.UseEnvVars {
				if err := flag.SetFlagsFromEnv(cmd.Parent().PersistentFlags(), true, cfg.EnvVarNamer, cfg.EnvVarPrefix, "P2PPipe"); err != nil {
					return err
				}
				if err := flag.SetFlagsFromEnv(cmd.PersistentFlags(), false, cfg.EnvVarNamer, cfg.EnvVarPrefix, "P2PPipe", "ListDiscoveredPeers"); err != nil {
					return err
				}
			}
			return client.RoundTrip(cmd.Context(), cfg, func(cc grpc.ClientConnInterface, in iocodec.Decoder, out iocodec.Encoder) error {
				cli := NewP2PPipeClient(cc)
				v := &ListDiscoveredPeersRequest{}

				if err := in(v); err != nil {
					return err
				}
				proto.Merge(v, req)

				res, err := cli.ListDiscoveredPeers(cmd.Context(), v)

				if err != nil {
					return err
				}

				return out(res)

			})
		},
	}

	flag.EnumVar(cmd.PersistentFlags(), &req.Method, cfg.FlagNamer("Method"), "")
	_Dht := &DHTDiscoveryArguments{}
	cmd.PersistentFlags().Bool(cfg.FlagNamer("Dht"), false, "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("Dht"), func() { req.Arguments = &ListDiscoveredPeersRequest_Dht{Dht: _Dht} })
	cmd.PersistentFlags().StringVar(&_Dht.Rv, cfg.FlagNamer("Dht Rv"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("Dht Rv"), func() { req.Arguments = &ListDiscoveredPeersRequest_Dht{Dht: _Dht} })

	return cmd
}

func _P2PPipeStartForwardingIOCommand(cfg *client.Config) *cobra.Command {
	req := &StartForwardingIORequest{}

	cmd := &cobra.Command{
		Use:   cfg.CommandNamer("StartForwardingIO"),
		Short: "StartForwardingIO RPC client",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.UseEnvVars {
				if err := flag.SetFlagsFromEnv(cmd.Parent().PersistentFlags(), true, cfg.EnvVarNamer, cfg.EnvVarPrefix, "P2PPipe"); err != nil {
					return err
				}
				if err := flag.SetFlagsFromEnv(cmd.PersistentFlags(), false, cfg.EnvVarNamer, cfg.EnvVarPrefix, "P2PPipe", "StartForwardingIO"); err != nil {
					return err
				}
			}
			return client.RoundTrip(cmd.Context(), cfg, func(cc grpc.ClientConnInterface, in iocodec.Decoder, out iocodec.Encoder) error {
				cli := NewP2PPipeClient(cc)
				v := &StartForwardingIORequest{}

				if err := in(v); err != nil {
					return err
				}
				proto.Merge(v, req)

				res, err := cli.StartForwardingIO(cmd.Context(), v)

				if err != nil {
					return err
				}

				return out(res)

			})
		},
	}

	_Peer := &Peer{}
	cmd.PersistentFlags().StringVar(&_Peer.Id, cfg.FlagNamer("Peer Id"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("Peer Id"), func() { req.Peer = _Peer })
	cmd.PersistentFlags().StringSliceVar(&_Peer.Addresses, cfg.FlagNamer("Peer Addresses"), nil, "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("Peer Addresses"), func() { req.Peer = _Peer })
	cmd.PersistentFlags().StringVar(&_Peer.Connectedness, cfg.FlagNamer("Peer Connectedness"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("Peer Connectedness"), func() { req.Peer = _Peer })
	flag.SliceVar(cmd.PersistentFlags(), flag.ParseMessageE[*Connection], &_Peer.Connections, cfg.FlagNamer("Peer Connections"), "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("Peer Connections"), func() { req.Peer = _Peer })
	_RemoteIo := &IO{}
	flag.EnumVar(cmd.PersistentFlags(), &_RemoteIo.IoType, cfg.FlagNamer("RemoteIo IoType"), "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("RemoteIo IoType"), func() { req.RemoteIo = _RemoteIo })
	_RemoteIo_Tcp := &IO_Tcp{}
	cmd.PersistentFlags().StringVar(&_RemoteIo_Tcp.Tcp, cfg.FlagNamer("RemoteIo Tcp"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("RemoteIo Tcp"), func() { req.RemoteIo = _RemoteIo; _RemoteIo.Io = _RemoteIo_Tcp })
	_RemoteIo_Udp := &IO_Udp{}
	cmd.PersistentFlags().StringVar(&_RemoteIo_Udp.Udp, cfg.FlagNamer("RemoteIo Udp"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("RemoteIo Udp"), func() { req.RemoteIo = _RemoteIo; _RemoteIo.Io = _RemoteIo_Udp })
	_RemoteIo_Unix := &IO_Unix{}
	cmd.PersistentFlags().StringVar(&_RemoteIo_Unix.Unix, cfg.FlagNamer("RemoteIo Unix"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("RemoteIo Unix"), func() { req.RemoteIo = _RemoteIo; _RemoteIo.Io = _RemoteIo_Unix })
	_LocalIo := &IO{}
	flag.EnumVar(cmd.PersistentFlags(), &_LocalIo.IoType, cfg.FlagNamer("LocalIo IoType"), "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("LocalIo IoType"), func() { req.LocalIo = _LocalIo })
	_LocalIo_Tcp := &IO_Tcp{}
	cmd.PersistentFlags().StringVar(&_LocalIo_Tcp.Tcp, cfg.FlagNamer("LocalIo Tcp"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("LocalIo Tcp"), func() { req.LocalIo = _LocalIo; _LocalIo.Io = _LocalIo_Tcp })
	_LocalIo_Udp := &IO_Udp{}
	cmd.PersistentFlags().StringVar(&_LocalIo_Udp.Udp, cfg.FlagNamer("LocalIo Udp"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("LocalIo Udp"), func() { req.LocalIo = _LocalIo; _LocalIo.Io = _LocalIo_Udp })
	_LocalIo_Unix := &IO_Unix{}
	cmd.PersistentFlags().StringVar(&_LocalIo_Unix.Unix, cfg.FlagNamer("LocalIo Unix"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("LocalIo Unix"), func() { req.LocalIo = _LocalIo; _LocalIo.Io = _LocalIo_Unix })

	return cmd
}

func _P2PPipeStopForwardingIOCommand(cfg *client.Config) *cobra.Command {
	req := &StopForwardingIORequest{}

	cmd := &cobra.Command{
		Use:   cfg.CommandNamer("StopForwardingIO"),
		Short: "StopForwardingIO RPC client",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.UseEnvVars {
				if err := flag.SetFlagsFromEnv(cmd.Parent().PersistentFlags(), true, cfg.EnvVarNamer, cfg.EnvVarPrefix, "P2PPipe"); err != nil {
					return err
				}
				if err := flag.SetFlagsFromEnv(cmd.PersistentFlags(), false, cfg.EnvVarNamer, cfg.EnvVarPrefix, "P2PPipe", "StopForwardingIO"); err != nil {
					return err
				}
			}
			return client.RoundTrip(cmd.Context(), cfg, func(cc grpc.ClientConnInterface, in iocodec.Decoder, out iocodec.Encoder) error {
				cli := NewP2PPipeClient(cc)
				v := &StopForwardingIORequest{}

				if err := in(v); err != nil {
					return err
				}
				proto.Merge(v, req)

				res, err := cli.StopForwardingIO(cmd.Context(), v)

				if err != nil {
					return err
				}

				return out(res)

			})
		},
	}

	_LocalIo := &IO{}
	flag.EnumVar(cmd.PersistentFlags(), &_LocalIo.IoType, cfg.FlagNamer("LocalIo IoType"), "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("LocalIo IoType"), func() { req.LocalIo = _LocalIo })
	_LocalIo_Tcp := &IO_Tcp{}
	cmd.PersistentFlags().StringVar(&_LocalIo_Tcp.Tcp, cfg.FlagNamer("LocalIo Tcp"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("LocalIo Tcp"), func() { req.LocalIo = _LocalIo; _LocalIo.Io = _LocalIo_Tcp })
	_LocalIo_Udp := &IO_Udp{}
	cmd.PersistentFlags().StringVar(&_LocalIo_Udp.Udp, cfg.FlagNamer("LocalIo Udp"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("LocalIo Udp"), func() { req.LocalIo = _LocalIo; _LocalIo.Io = _LocalIo_Udp })
	_LocalIo_Unix := &IO_Unix{}
	cmd.PersistentFlags().StringVar(&_LocalIo_Unix.Unix, cfg.FlagNamer("LocalIo Unix"), "", "")
	flag.WithPostSetHook(cmd.PersistentFlags(), cfg.FlagNamer("LocalIo Unix"), func() { req.LocalIo = _LocalIo; _LocalIo.Io = _LocalIo_Unix })

	return cmd
}
