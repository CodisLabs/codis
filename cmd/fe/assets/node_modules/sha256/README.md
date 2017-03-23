# sha256

JavaScript component to compute the SHA256 of strings or bytes.

## DEPRECATION NOTICE

This library is deprecated. If you think it's important to maintain this library, please submit an issue and the case will be consider or the repo / ownership will be transferred to you. Viable alternatives:

1. Use Browserify
2. https://www.npmjs.com/package/sha.js (used by Browserify)
3. https://github.com/indutny/hash.js


## Install

### Node.js/Browserify

    npm install --save sha256

Note: when Node.js is detected, the Node [`Crypto`](http://nodejs.org/api/crypto.html) component is used instead of re-implementing the SHA256 hash logic.



## Usage

There are two methods, one for computing the hash of the input, and one for double-hashing it:

```js
sha256('hello');   // "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
sha256.x2('hello'); // "d7914fe546b684688bb95f4f888a92dfc680603a75f23eb823658031fff766d9"
```

Input is either an array of bytes or a string. **String are always interpreted as binary data**; if you have a hex-encoded string of data to parse, first convert it to a binary string or array of bytes.

Output by default is a hexadecimal-encoded string. Other options are an array of bytes, or a binary-encoded string:

```js
sha256('hello');   // "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" <= Hex-encoded; default
sha256('hello', { asBytes: true }); // [44,242,77,186,95,176,163,14,38,232,59,42,197,185,226,158,27,22,30,92,31,167,66,94,115,4,51,98,147,139,152,36] <= Array of bytes
sha256('hello', { asString: true }); // ",òMº_°£&è;*Å¹â\§B^s3b$" <= Binary-encoded string
```

## Test

Unit tests are written in [Mocha](http://visionmedia.github.io/mocha/). To run the test suite, checkout the git repository, and from within the base folder run:

```sh
$ npm install --dev
$ npm test
```

# Credits

Most of the code from CryptoJS https://code.google.com/p/crypto-js/

# License

(MIT License)

Copyright 2013, JP Richardson  <jprichardson@gmail.com>