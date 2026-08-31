# gosu-api

Wrapper for the osu! API v2.

## Install

```bash
go get github.com/minoxs/gosu-api
```

## Usage

```go
var c = gosu.NewClient()

var token, err = c.GetGuestToken(gosu.Credentials{ClientID: id, ClientSecret: secret})
if err != nil { return err }

var user, _ = c.GetUserByName(token, "peppy")
```

## Rate limiting

Please be mindful of your usage of the osu! API. It is provided for free, but hosting something so detailed like this is not easy on the infrastructure.

Because of this, I decided to make rate limiting a first class citizen on this package. You can rate limit requests on a configurable value, but as the API terms states:
> Please limit your usage to no more than 60 requests per minute (generally 1 request per second).
> The internal rate limits are higher than this and allow some degree of bursting, but exceeding this specified limit may lead to your API tokens being revoked, or in serious abuse cases your access to the API being restricted.

You can create multiple clients being fed from the same limiter and separate request priorities accordingly to make sure that your application gets the data it needs.
Requests with higher priority will be answered first, but never exceeding the configured limit.

```go
var limiter = gosu.NewRateLimiter(nil, 60)

// Requests sent via the high client will be answered first
var high = gosu.NewClientWith(limiter, 1)
var low = gosu.NewClientWith(limiter, 0)
// Multiple requests on the same client will be queued in order
```

**You have been warned. If you use this package incorrectly and get banned from the API, you're on your own.**

## Coverage

I am building this on a best-effort basis mostly to get what I need done first. If some endpoint is missing and you'd like it covered, feel free to send a PR or file an issue.

| Section      | Support                            |
|--------------|------------------------------------|
| OAuth Tokens | Client credentials grant.          |
| Users        | Fetch user data and recent scores. |
| Beatmaps     | Fetch beatmap metadata.            |
| Scores       | Fetch recent scores.               |

**osu!standard only.**

## Plugins

This package is built to wrap the osu! API with the least amount of friction possible, but keeping it minimal.
I would like to have first class support to caching in this package, but it is not something I found a way to cleanly add without baking lots of assumptions of how this package will be used.
So I am trying to keep things extensible enough to allow for plugins to be built on top of this. If you build something based off of this and want it listed here, let me know.

| Name                                                                | Description                                                                   |
|---------------------------------------------------------------------|-------------------------------------------------------------------------------|
| [gosu-score-tracker](https://github.com/Minoxs/gosu-scores-tracker) | Plugin to track user scores. Extracted from earlier versions of this package. |

## AI disclosure

Most of this code is pure human made slop, but the more recent versions have had a hefty usage of AI to quickly add new endpoints and rate limiting.
To be honest, I hate it probably as much as you do, but a few years working as a software engineer and your hopes and dreams will absolutely be crushed as well.
Still, I had a lot of fun designing this thing and have reviewed things thoroughly, once I get things at a stable point I will make sure everything is cleaned up.
