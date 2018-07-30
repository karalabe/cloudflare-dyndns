// CloudFlare Dynamic DNS Updater
// Copyright (c) 2015 Péter Szilágyi. All rights reserved.
//
// Released under the MIT license.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

var (
	updateFlag  = flag.Duration("update", time.Minute, "Time interval to run the updater")
	userFlag    = flag.String("user", "", "CloudFlare username to update with")
	keyFlag     = flag.String("key", "", "CloudFlare authorization token")
	domainsFlag = flag.String("domains", "", "Comma separated domain list to update")
	ttlFlag     = flag.Int("ttl", 120, "Domain time to live value")
)

var (
	domainSplitter = regexp.MustCompile(".+\\.(.+\\..+)")
)

func main() {
	flag.Parse()

	previous := "" // Previous address to prevent hammering CloudFlare
	for {
		// Resolve the external address and update if valid
		address, err := resolveAddress()
		if err != nil {
			log.Printf("Failed to resolve external address: %v", err)
		}
		if address != "" && address != previous {
			log.Printf("Updating IP address to %s", address)

			for _, host := range strings.Split(*domainsFlag, ",") {
				if err := updateDNS(address, *userFlag, *keyFlag, host, *ttlFlag); err != nil {
					log.Printf("Failed to update %s: %v", host, err)
					continue
				}
				log.Printf("Domain updated: %s", host)
				previous = address
			}
		}
		// Wait for the next invocation
		time.Sleep(*updateFlag)
	}
}

// resolveAddress tries to resolve the external IP address of the machine via
// third party resolution services. Currently two are queried and the DNS entry
// only updated if they both match.
func resolveAddress() (string, error) {
	// Resolve the external address via whatismyipaddress.com
	reply, err := http.Get("http://ipv4bot.whatismyipaddress.com")
	if err != nil {
		return "", err
	}
	defer reply.Body.Close()

	potential, err := ioutil.ReadAll(reply.Body)
	if err != nil {
		return "", err
	}
	// Resolve the external address via ipify.org
	reply, err = http.Get("https://api.ipify.org")
	if err != nil {
		return "", err
	}
	defer reply.Body.Close()

	confirm, err := ioutil.ReadAll(reply.Body)
	if err != nil {
		return "", err
	}
	// Confirm or discard the resolution
	if bytes.Compare(potential, confirm) != 0 {
		return "", fmt.Errorf("resolution conflict: %s != %s", string(potential), string(confirm))
	}
	return string(potential), nil
}

// updateDNS updates a single CloudFlare DNS entry to the given IP address.
func updateDNS(address string, user, key string, host string, ttl int) error {
	// Split the domain into zone and record fields
	domain := domainSplitter.FindStringSubmatch(host)[1]

	// Create an authenticated Cloudflare client
	api, err := cloudflare.New(key, user)
	if err != nil {
		return err
	}
	// Resolve the zone and record id for the host
	zone, err := api.ZoneIDByName(domain)
	if err != nil {
		fmt.Errorf("zone id resolution failed: %v", err)
	}
	recs, err := api.DNSRecords(zone, cloudflare.DNSRecord{Name: host, Type: "A"})
	if err != nil {
		fmt.Errorf("record id resolution failed: %v", err)
	}
	if len(recs) != 1 {
		fmt.Errorf("invalid number of DNS records found: %+v", recs)
	}
	record := recs[0]

	// Post the Cloudflare dns update
	record.Content = address
	record.TTL = ttl

	if err := api.UpdateDNSRecord(zone, record.ID, record); err != nil {
		return fmt.Errorf("dns record update failed: %v", err)
	}
	return nil
}
