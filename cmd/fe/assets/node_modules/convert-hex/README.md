convert-hex
===========

JavaScript component to convert to/from hex strings and byte arrays.

AMD/CommonJS compatible.


Install
-------

### Node.js/Browserify

    npm install --save cryptocoin-convert-hex


### Component

    component install cryptocoin/convert-hex


### Bower

    bower install cryptocoin/convert-hex


### Script

```html
<script src="/path/to/convert-hex.js"></script>
```


Usage
-----

(if using script, the global is `convertHex`)

### bytesToHex(bytes)

```js
var convertHex = require('convert-hex')

var bytes = [0x34, 0x55, 0x1, 0xDF]
console.log(convertHex.bytesToHex(bytes)) //"345501df"
```


### hexToBytes(hexStr)

```js
var hex = "34550122DF" //"0x" prefix is optional
console.dir(conv.hexToBytes(hex).join(',')) //'[52,85,1,34,223]'
```

Credits
-------

Loosely inspired by code from here: https://github.com/vbuterin/bitcoinjs-lib & CryptoJS


License
-------

(MIT License)

Copyright 2013, JP Richardson  <jprichardson@gmail.com>