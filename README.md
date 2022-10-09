# iphook

This tool watches a Linux system's public IPv4 and IPv6 addresses using netlink.
When an address changes, it will make a configurable HTTP request (webhook) submitting the current IPv4 and IPv6 addresses.

It can be used to update [DynDNS2](https://help.dyn.com/remote-access-api/) compatible dynamic dns services.

## installation

* [Install go](https://help.dyn.com/remote-access-api/)
* `go install github.com/markuslindenberg/iphook@latest

## usage

* Set environment variables `IPHOOK_USER` and `IPHOOK_PASSWORD` for HTTP authentication
* Use `-url` parameter to provide your Dyndns server's update URL and optional static parameters.
* Example command line: `iphook -url "https://dyndns.example.com/nic/update?hostname=test.example.com"`

See `iphook -h` for advanced options.
