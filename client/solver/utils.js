const createHash = require('create-hash');

const writeUInt32 = (buffer, value, off) => {
  buffer.writeUInt32LE(value, off);
  return off + 4;
}

const writeTimestamp = (buffer, ts, off) => {
  const high = (ts / 0x100000000) >>> 0;
  const low = (ts & 0xffffffff) >>> 0;
  buffer.writeUInt32BE(high, off);
  buffer.writeUInt32BE(low, off + 4);
  return off + 8;
}

const hash = (nonce, prefix) => {
  const h = createHash('sha256');
  if (prefix) h.update(prefix, 'hex');
  h.update(nonce);
  return h.digest();
}

const checkComplexity = (hash, complexity) => {
  if (complexity >= hash.length * 8) {
    throw new Error('Complexity is too high');
  }
  let off = 0;
  let i = 0;
  for (i = 0; i <= complexity - 8; i += 8, off++) {
    if (hash[off] !== 0) return false;
  }

  const mask = 0xff << (8 + i - complexity);
  return (hash[off] & mask) === 0;
}

module.exports = {
  writeTimestamp,
  writeUInt32,
  hash,
  checkComplexity
};