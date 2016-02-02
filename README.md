# CloudFlare Dynamic DNS Updater

This is an extremely simple tool to update CloudFlare dynamic DNS entries. It's
sole purpose is to periodically query the external IP address of the machine it
is running on, and modify the DNS entry accordingly. Opposed to a few scripts
out there that do a one shot update requiring regular cron sheduling, this one
is more aimed to environments where you can configure it as a service and leave
the updater running in the background.

## Installation and usage

The tool in written in Go and follows the usual installation method:

```
go get github.com/karalabe/cloudflare-dyndns
```

```
cloudflare-dyndns --help

Usage of cloudflare-dyndns:
  -domains string
      Comma separated domain list to update
  -key string
      CloudFlare authorization token
  -ttl int
      Domain time to live value (default 120)
  -update duration
      Time interval to run the updater (default 1m0s)
  -user string
      CloudFlare username to update with
```
