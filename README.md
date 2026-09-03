# gosu-api

Wrapper for the osu! API v2.

## Install

```bash
go get github.com/minoxs/gosu-api
```

## Usage

```go
// Rate limit defaults to 60 req/min
var app = gosu.NewApp(id, secret)
// Pass gosu.RateLimit(n) and/or gosu.Transport(http) to change client behaviour

// Check if the credentials are valid
if err := app.Validate(); err != nil { log.Fatal(err) }

// Request away :)
var user, err = app.GuestClient(0).GetUserByName("peppy")
if err != nil { return err }
```

### OAuth

User-scoped endpoints need a resource-owner token. Send the user to the authorize URL, exchange the code osu! returns for a token, then build a `ResourceClient`.

```go
var app = gosu.NewApp(id, secret)

var url = app.OAuth().AuthorizeURL(redirectURI, []gosu.Scope{gosu.ScopeIdentify}, csrf)
// Send the user to url, receive code on the redirect

var token, err = app.OAuth().Exchange(code, redirectURI)
if err != nil { return err }

var me, _ = app.ResourceOwnerClient(0, token).GetOwnUser()
```

## Rate limiting

Please be mindful of your usage of the osu! API. It is provided for free, but hosting something so detailed like this is not easy on the infrastructure.

Because of this, I decided to make rate limiting a first class citizen on this package. You can rate limit requests on a configurable value, but as the API terms states:
> Please limit your usage to no more than 60 requests per minute (generally 1 request per second).
> The internal rate limits are higher than this and allow some degree of bursting, but exceeding this specified limit may lead to your API tokens being revoked, or in serious abuse cases your access to the API being restricted.

I have provided `NewApp()` to abstract away all of this, exposing the request priority so you can make sure that your application gets the data it needs.
Requests with higher priority will be answered first, but never exceeding the configured limit.

```go
var high = app.GuestClient(1)
var low = app.GuestClient(0)
```

If you have some complex requirements, you can reach into the internal bits of this library to mix and match its building blocks, I am trying to make sure things can be composed cleanly.
**However, you have been warned. If you use this package incorrectly and get banned from the API, you're on your own.**

## Coverage

I am building this on a best-effort basis mostly to get what I need done first. If some endpoint is missing and you'd like it covered, feel free to send a PR or file an issue.

| Section      | Support                                                     |
|--------------|-------------------------------------------------------------|
| OAuth Tokens | Client credentials, authorization code, and refresh grants. |
| Users        | Fetch user data and recent scores.                          |
| Beatmaps     | Fetch beatmap metadata.                                     |
| Scores       | Fetch recent scores.                                        |

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
