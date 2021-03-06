package maddy

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"

	"github.com/mholt/caddy/caddyfile"
)

type server interface {
	Serve(net.Listener) error
}

var Directives = []string{
	"bind",
	"tls",

	"log",
	"errors",
	"compress",
	"proxy",
	"pgp",
	"debug",
	"auth",
}

func Start(blocks []caddyfile.ServerBlock) error {
	done := make(chan error, 1)

	for _, block := range blocks {
		var adresses []Address
		var proto string
		tlsConfig := new(tls.Config)
		for _, k := range block.Keys {
			addr, err := standardizeAddress(k)
			if err != nil {
				return fmt.Errorf("Cannot parse block key %q: %v", k, err)
			}

			p := addr.Protocol()
			if proto == "" {
				proto = p
			} else if proto != p {
				return fmt.Errorf("Block contains incompatible protocols: %s and %s", proto, p)
			}

			if addr.IsTLS() && tlsConfig.ServerName == "" {
				tlsConfig.ServerName = addr.Host
			}

			adresses = append(adresses, addr)
		}

		var s server
		var err error
		switch proto {
		case "imap":
			s, err = newIMAPServer(block.Tokens)
		case "smtp":
			s, err = newSMTPServer(block.Tokens)
		default:
			return fmt.Errorf("Unsupported protocol %q", proto)
		}
		if err != nil {
			return err
		}

		if tokens, ok := block.Tokens["tls"]; ok {
			if err := setTLS(caddyfile.NewDispenserTokens("", tokens), &tlsConfig); err != nil {
				return err
			}
		} else {
			tlsConfig = nil
		}

		for _, addr := range adresses {
			var l net.Listener
			var err error
			l, err = net.Listen(addr.Network(), addr.Address())
			if err != nil {
				return fmt.Errorf("Cannot listen: %v", err)
			}

			if addr.IsTLS() {
				l = tls.NewListener(l, tlsConfig)
			}

			log.Printf("%s server listening on %s\n", addr.Scheme, l.Addr().String())
			go func() {
				done <- s.Serve(l)
			}()
		}
	}

	return <-done
}
