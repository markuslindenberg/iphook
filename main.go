package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func init() {
	log.SetFlags(0)
}

func main() {
	var (
		ifname        = flag.String("interface", "", "interface name (empty: auto)")
		interval      = flag.Duration("interval", 5*time.Second, "check interval")
		errorInterval = flag.Duration("interval.error", 5*time.Minute, "wait time after an error response")
		urlstring     = flag.String("url", "/nic/update?hostname=test.example.com", "dynamic dns update url")
		ipv4param     = flag.String("param.ipv4", "myip", "http parameter for ipv4 address")
		ipv6param     = flag.String("param.ipv6", "myipv6", "http parameter for ipv6 address")

		err              error
		hookURL          *url.URL
		originalRawQuery string
	)
	flag.Parse()

	hookURL, err = url.Parse(*urlstring)
	if err != nil {
		log.Fatal(err)
	}
	if username := os.Getenv("IPHOOK_USER"); username != "" {
		hookURL.User = url.UserPassword(username, os.Getenv("IPHOOK_PASSWORD"))
	}
	originalRawQuery = hookURL.RawQuery

	var link netlink.Link
	if *ifname != "" {
		var err error
		link, err = netlink.LinkByName(*ifname)
		if err != nil {
			log.Fatal(err)
		}
	}

	var submittedIPv4, submittedIPv6 net.IP
	for {
		ipv4, err := getAddress(link, netlink.FAMILY_V4)
		if err != nil {
			log.Println("ipv4:", err)
		}

		ipv6, err := getAddress(link, netlink.FAMILY_V6)
		if err != nil {
			log.Println("ipv6:", err)
		}

		if !(ipv4 == nil && ipv6 == nil) && !(ipv4.Equal(submittedIPv4) && ipv6.Equal(submittedIPv6)) {
			log.Println("addresses changed. ipv4:", ipv4, "ipv6:", ipv6)

			query, _ := url.ParseQuery(originalRawQuery)
			if *ipv4param != "" && ipv4 != nil {
				query.Add(*ipv4param, ipv4.String())
			}
			if *ipv6param != "" && ipv6 != nil {
				query.Add(*ipv6param, ipv6.String())
			}
			hookURL.RawQuery = query.Encode()
			log.Println("request: GET", hookURL.Redacted())

			if hookURL.Host == "" {
				log.Println("warning: host missing in url, skipping request")
			} else {
				var response *http.Response
				response, err = http.Get(hookURL.String())
				if err != nil {
					log.Println("request error:", err)
				} else {
					if response.StatusCode != 200 {
						io.Copy(io.Discard, response.Body)
						response.Body.Close()
						log.Println("response error:", response.Status)
						time.Sleep(*errorInterval)
						continue
					} else {
						body, _ := io.ReadAll(response.Body)
						response.Body.Close()
						reply := strings.TrimSpace(string(body))
						log.Printf("response: %s", reply)

						submittedIPv4 = ipv4
						submittedIPv6 = ipv6
					}
				}
			}
		}

		time.Sleep(*interval)
	}
}

// getAddress uses netlink to find the system's public ip address
func getAddress(link netlink.Link, family int) (address net.IP, err error) {
	if link == nil {
		// find link w/ default route
		routes, err := netlink.RouteList(link, family)
		if err != nil {
			return nil, err
		}

		defaultRoutes := []netlink.Route{}
		for _, r := range routes {
			// route is default route
			if r.Dst == nil {
				defaultRoutes = append(defaultRoutes, r)
			}
		}

		switch len(defaultRoutes) {
		case 0:
			return nil, nil
		case 1:
			link, err = netlink.LinkByIndex(defaultRoutes[0].LinkIndex)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("found %d default routes, expecting 1", len(defaultRoutes))
		}
	}

	addrs, err := netlink.AddrList(link, family)
	if err != nil {
		return nil, err
	}

	// filter unsuitable addresses
	filteredAddresses := []netlink.Addr{}
	for _, a := range addrs {
		// only scope global
		if a.Scope != int(netlink.SCOPE_UNIVERSE) {
			continue
		}
		// skip temporary addresses
		if a.Flags&unix.IFA_F_TEMPORARY != 0 {
			continue
		}
		// skip private addresses
		if a.IP.IsPrivate() {
			continue
		}

		filteredAddresses = append(filteredAddresses, a)
	}

	switch len(filteredAddresses) {
	case 0:
		return nil, nil
	case 1:
		return filteredAddresses[0].IP, nil
	default:
		return nil, fmt.Errorf("found %d addresses, expecting 1", len(filteredAddresses))
	}
}
