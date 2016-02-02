// CloudFlare Dynamic DNS Updater
// Copyright (c) 2015 Péter Szilágyi. All rights reserved.
//
// Released under the MIT license.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	updateFlag  = flag.Duration("update", time.Minute, "Time interval to run the updater")
	userFlag    = flag.String("user", "", "CloudFlare username to update with")
	keyFlag     = flag.String("key", "", "CloudFlare authorization token")
	domainsFlag = flag.String("domains", "", "Comma separated domain list to update")
	ttlFlag     = flag.Int("ttl", 120, "Domain time to live value")
)

var (
	domainSplitter = regexp.MustCompile("(.+)\\.(.+\\..+)")
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
				if _, err := updateDNS(address, *userFlag, *keyFlag, host, *ttlFlag); err != nil {
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
func updateDNS(address string, user, key string, domain string, ttl int) (string, error) {
	// Split the domain into zone and record fields
	parts := domainSplitter.FindStringSubmatch(domain)
	zone, record := parts[2], parts[1]

	// Resolve the record id for the host
	id, err := resolveRecordId(user, key, zone, record)
	if err != nil {
		return "", fmt.Errorf("record id reolution failed: %v", err)
	}
	// Post the CloudFlare DNS update request
	reply, err := http.PostForm("https://www.cloudflare.com/api_json.html", url.Values{
		"a":       {"rec_edit"},
		"email":   {user},
		"tkn":     {key},
		"z":       {zone},
		"id":      {id},
		"type":    {"A"},
		"name":    {record},
		"ttl":     {strconv.Itoa(ttl)},
		"content": {address},
	})
	if err != nil {
		return "", err
	}
	defer reply.Body.Close()

	// Parse the reply and check if an error occurred
	body, err := ioutil.ReadAll(reply.Body)
	if err != nil {
		return "", err
	}
	var failure struct {
		Result  string `json:"result"`
		Message string `json:"msg"`
	}
	if err := json.Unmarshal(body, &failure); err != nil {
		return "", err
	}
	if failure.Result == "error" {
		return "", fmt.Errorf("request denied: %s", failure.Message)
	}
	// Yay, no failure, flatten the reply and return
	return string(body), err
}

// resolveRecordId resolves the id string of a single subdomain entry in a zone
// listing.
func resolveRecordId(user, key string, zone, record string) (string, error) {
	// Post a CloudFlare DNS record list request
	reply, err := http.PostForm("https://www.cloudflare.com/api_json.html", url.Values{
		"a":     {"rec_load_all"},
		"email": {user},
		"tkn":   {key},
		"z":     {zone},
	})
	if err != nil {
		return "", err
	}
	defer reply.Body.Close()

	// Parse the reply and check if an error occurred
	body, err := ioutil.ReadAll(reply.Body)
	if err != nil {
		return "", err
	}
	var response struct {
		Result   string `json:"result"`
		Message  string `json:"msg"`
		Response struct {
			Records struct {
				Objs []struct {
					Id   string `json:"rec_id"`
					Name string `json:"display_name"`
				} `json:"objs"`
			} `json:"recs"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	if response.Result == "error" {
		return "", fmt.Errorf("request denied: %s", response.Message)
	}
	// Find the DNS record in the response
	for _, obj := range response.Response.Records.Objs {
		if obj.Name == record {
			return obj.Id, nil
		}
	}
	return "", errors.New("unknown record")
}
