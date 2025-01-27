const utils = require('./utils.js');

const minNonceSize = 8
const nonceSize = minNonceSize + 8

class Solver {
  solve(
    complexity,
    prefix
  ){
    const nonce = Buffer.alloc(nonceSize)
    while (true) {
      this.genNonce(nonce)
      const hash = utils.hash(nonce, prefix)
      if (utils.checkComplexity(hash, complexity)) {
        return nonce
      }
    }
  }
  genNonce(buf){
    const now = Date.now()
    let off = utils.writeTimestamp(buf, now, 0)
    const words = off + (((buf.length - off) / 4) | 0) * 4
    for (; off < words; off += 4) {
      utils.writeUInt32(buf, (Math.random() * 0x100000000) >>> 0, off)
    }
    for (; off < buf.length; off++) {
      buf[off] = (Math.random() * 0x100) >>> 0
    }
  }
}

module.exports = { Solver }
