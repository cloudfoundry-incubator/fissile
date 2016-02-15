BOSH release to run Tor
=======================

Background
----------

### What is Tor?

Tor is free software and an open network that helps you defend against traffic analysis, a form of network surveillance that threatens personal freedom and privacy, confidential business activities and relationships, and state security.

### Why Anonymity Matters

Tor protects you by bouncing your communications around a distributed network of relays run by volunteers all around the world: it prevents somebody watching your Internet connection from learning what sites you visit, and it prevents the sites you visit from learning your physical location.

Usage
-----

To use this bosh release, first upload it to your BOSH:

```
bosh upload release https://bosh.io/d/github.com/cloudfoundry-community/tor-boshrelease
```

For [bosh-lite](https://github.com/cloudfoundry/bosh-lite), you can quickly create a deployment manifest & deploy a cluster:

```
templates/make_manifest warden
bosh -n deploy
```

Next, run the `new-hostname` errand - it is a one-time task to generate a new random Onion hostname and private key:

```
bosh run errand new_hostname
```

The output will include the `hostname` `xxxxxxx.onion` and the accompanying `private_key`:

```
==> hostname <==
3ngzbk47ntwpv7dr.onion

==> private_key <==
-----BEGIN RSA PRIVATE KEY-----
MIICXQIBAAKBgQDpiZjNB5rmonRIlB6uy34RZbc2z46UyeHYOa574e3tFeOryTp5
Z4T2siA7uOG0LtPoFMziXc+qp4sdHaC0hp01pxQzEWDKEACg0C1x5WT59o/Dlocv
SZuqD7aUkQ+kXt9zfQ0t3sANkpuESvM3qXV4+hzx6PbPqTVJkCHtnVzCtQIDAQAB
AoGAdTOixbK9YGXDKfF7/IkPebesXQuJKM6wUw2PrYhTGZrUqY/RksALEKuQVaiR
TRX7LwvRTwF5iNGQlUobLr4oAqFsCX8xJMXwsMwBC21yH80+Z6WA4CtPb87f9gVR
XQvBYypzn9KvhW4Jllf6mmg4JrZSLNpedzhDTLMQFiyBeIECQQD/6Fa9CDFarwHI
Lg8qdANF5CRTfwJAGmhnWwKKGnnbwn294JmCna84Zwe8fB5g9bjopUij6C6gJvVX
7hbm9lGlAkEA6Z8wkZjvdHiAf7WaMRk9zicbyIfj9sfkGOjBqv4NULQjz5daCxjr
IOkPrxoZJvqVIXKnsXiozDNLEDRjttK/0QJBALpLDCHOfgdTEYwFo7q2+878V0mF
U0EROGHNShr5TS6i9mCsyXPhkLYRovserArPts19zVSs6Ixj8AUT6Q430JUCQQDR
...
-----END RSA PRIVATE KEY-----
Errand new_hostname is complete; exit status 0
```

Create a YAML file `tmp/hostname.yml` that contains the values from the errand output

```
jobs:
  tor:
    instances: 1
    properties:
      tor:
        hostname: 3ngzbk47ntwpv7dr.onion
        private_key: |
          -----BEGIN RSA PRIVATE KEY-----
          MIICXQIBAAKBgQDpiZjNB5rmonRIlB6uy34RZbc2z46UyeHYOa574e3tFeOryTp5
          Z4T2siA7uOG0LtPoFMziXc+qp4sdHaC0hp01pxQzEWDKEACg0C1x5WT59o/Dlocv
          SZuqD7aUkQ+kXt9zfQ0t3sANkpuESvM3qXV4+hzx6PbPqTVJkCHtnVzCtQIDAQAB
          AoGAdTOixbK9YGXDKfF7/IkPebesXQuJKM6wUw2PrYhTGZrUqY/RksALEKuQVaiR
          TRX7LwvRTwF5iNGQlUobLr4oAqFsCX8xJMXwsMwBC21yH80+Z6WA4CtPb87f9gVR
          XQvBYypzn9KvhW4Jllf6mmg4JrZSLNpedzhDTLMQFiyBeIECQQD/6Fa9CDFarwHI
          Lg8qdANF5CRTfwJAGmhnWwKKGnnbwn294JmCna84Zwe8fB5g9bjopUij6C6gJvVX
          7hbm9lGlAkEA6Z8wkZjvdHiAf7WaMRk9zicbyIfj9sfkGOjBqv4NULQjz5daCxjr
          IOkPrxoZJvqVIXKnsXiozDNLEDRjttK/0QJBALpLDCHOfgdTEYwFo7q2+878V0mF
          U0EROGHNShr5TS6i9mCsyXPhkLYRovserArPts19zVSs6Ixj8AUT6Q430JUCQQDR
          ...
          -----END RSA PRIVATE KEY-----
```

Now regenerate the BOSH manifest to include your bespoke `hostname`:

```
templates/make_manifest warden tmp/hostname.yml
bosh -n deploy
```

You will now have a single container running Tor.

It doesn't do much right now as it is configured to route traffic to port 80, but there is nothing running on port 80.

### Development

-	Pipeline at http://ci.starkandwayne.com:8080/pipelines/tor-boshrelease

As a developer of this release, create new releases and upload them:

```
bosh create release --force && bosh -n upload release
```
