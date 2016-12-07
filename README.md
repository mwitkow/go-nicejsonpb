# Nice Golang Protobuf JSONPB 

[![Travis Build](https://travis-ci.org/mwitkow/go-proto-validators.svg)](https://travis-ci.org/mwitkow/go-proto-validators)
[![Apache 2.0 License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

The `jsonpb` implementation of [golang/protobuf](https://github.com/golang/protobuf) 
has very bad error handling, making it hard to return human-understandable errors. 

This is a fork of the `Unmarshal` functionality of `jsonpb` with fixes for error handling:
 * Errors are are now prefixed with a "stack" path of fields that are returned in
 * Poor "cannot deserialize into `json.RawMessage" errors now use proper types
 * Unknown fields now return a helpful message listing known fields :)