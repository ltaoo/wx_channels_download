const MASK_64_BITS = (1n << 64n) - 1n;
const GOLDEN_GAMMA = 0x9e3779b97f4a7c13n;

function wrap_64(value) {
  return value & MASK_64_BITS;
}

function mix_64(values) {
  values[0] = wrap_64(values[0] - values[4]);
  values[5] ^= values[7] >> 9n;
  values[7] = wrap_64(values[7] + values[0]);
  values[1] = wrap_64(values[1] - values[5]);
  values[6] = wrap_64(values[6] ^ (values[0] << 9n));
  values[0] = wrap_64(values[0] + values[1]);
  values[2] = wrap_64(values[2] - values[6]);
  values[7] ^= values[1] >> 23n;
  values[1] = wrap_64(values[1] + values[2]);
  values[3] = wrap_64(values[3] - values[7]);
  values[0] = wrap_64(values[0] ^ (values[2] << 15n));
  values[2] = wrap_64(values[2] + values[3]);
  values[4] = wrap_64(values[4] - values[0]);
  values[1] ^= values[3] >> 14n;
  values[3] = wrap_64(values[3] + values[4]);
  values[5] = wrap_64(values[5] - values[1]);
  values[2] = wrap_64(values[2] ^ (values[4] << 20n));
  values[4] = wrap_64(values[4] + values[5]);
  values[6] = wrap_64(values[6] - values[2]);
  values[3] ^= values[5] >> 17n;
  values[5] = wrap_64(values[5] + values[6]);
  values[7] = wrap_64(values[7] - values[3]);
  values[4] = wrap_64(values[4] ^ (values[6] << 14n));
  values[6] = wrap_64(values[6] + values[7]);
}

function is_aac_64(ctx) {
  ctx.cc = wrap_64(ctx.cc + 1n);
  ctx.bb = wrap_64(ctx.bb + ctx.cc);

  for (let i = 0; i < 256; i += 1) {
    switch (i % 4) {
      case 0:
        ctx.aa = wrap_64(~(ctx.aa ^ (ctx.aa << 21n)));
        break;
      case 1:
        ctx.aa ^= ctx.aa >> 5n;
        break;
      case 2:
        ctx.aa = wrap_64(ctx.aa ^ (ctx.aa << 12n));
        break;
      default:
        ctx.aa ^= ctx.aa >> 33n;
    }

    ctx.aa = wrap_64(ctx.aa + ctx.mm[(i + 128) % 256]);
    const x = ctx.mm[i];
    const y = wrap_64(ctx.mm[Number((x >> 3n) % 256n)] + ctx.aa + ctx.bb);
    ctx.mm[i] = y;
    ctx.bb = wrap_64(ctx.mm[Number((y >> 11n) % 256n)] + x);
    ctx.seed[i] = ctx.bb;
  }
}

function rand_64_init(key) {
  const ctx = {
    rand_cnt: 255,
    seed: new Array(256).fill(0n),
    mm: new Array(256).fill(0n),
    aa: 0n,
    bb: 0n,
    cc: 0n,
  };
  const values = new Array(8).fill(GOLDEN_GAMMA);
  ctx.seed[0] = key;

  for (let i = 0; i < 4; i += 1) mix_64(values);
  for (let i = 0; i < 256; i += 8) {
    for (let j = 0; j < 8; j += 1) {
      values[j] = wrap_64(values[j] + ctx.seed[i + j]);
    }
    mix_64(values);
    for (let j = 0; j < 8; j += 1) ctx.mm[i + j] = values[j];
  }
  for (let i = 0; i < 256; i += 8) {
    for (let j = 0; j < 8; j += 1) {
      values[j] = wrap_64(values[j] + ctx.mm[i + j]);
    }
    mix_64(values);
    for (let j = 0; j < 8; j += 1) ctx.mm[i + j] = values[j];
  }

  is_aac_64(ctx);
  return ctx;
}

function isaac_random(ctx) {
  const result = ctx.seed[ctx.rand_cnt];
  if (ctx.rand_cnt === 0) {
    is_aac_64(ctx);
    ctx.rand_cnt = 255;
  } else {
    ctx.rand_cnt -= 1;
  }
  return result;
}

function create_wxchannels_decryptor(
  key,
  offset = 0,
  encrypted_length = 131072,
) {
  if (typeof key !== "bigint") {
    throw new TypeError("key must be bigint");
  }
  if (
    !Number.isSafeInteger(offset) ||
    offset < 0 ||
    !Number.isSafeInteger(encrypted_length) ||
    encrypted_length < 0
  ) {
    throw new RangeError("offset and encrypted_length must be safe integers");
  }

  const ctx = rand_64_init(key);
  const random_bytes = new Uint8Array(8);
  const random_view = new DataView(random_bytes.buffer);
  let consumed = 0;
  let ks_pos = 8;

  if (encrypted_length > 0 && offset < encrypted_length) {
    consumed = offset;
    const skip_blocks = Math.floor(offset / 8);
    for (let block = 0; block < skip_blocks; block += 1) isaac_random(ctx);
    random_view.setBigUint64(0, isaac_random(ctx), false);
    ks_pos = offset % 8;
  } else {
    consumed = encrypted_length;
  }

  return {
    decrypt_chunk(data) {
      if (!(data instanceof Uint8Array)) {
        throw new TypeError("data must be Uint8Array");
      }
      if (encrypted_length === 0 || consumed >= encrypted_length) return data;

      const decrypt_length = Math.min(
        data.length,
        encrypted_length - consumed,
      );
      for (let index = 0; index < decrypt_length; index += 1) {
        if (ks_pos >= 8) {
          random_view.setBigUint64(0, isaac_random(ctx), false);
          ks_pos = 0;
        }
        data[index] ^= random_bytes[ks_pos];
        ks_pos += 1;
      }
      consumed += decrypt_length;
      return data;
    },
  };
}

function decrypt_wxchannels_data(data, key, encrypted_length = 131072) {
  if (!(data instanceof Uint8Array)) {
    throw new TypeError("data must be Uint8Array");
  }
  return create_wxchannels_decryptor(key, 0, encrypted_length).decrypt_chunk(
    data,
  );
}

globalThis.create_wxchannels_decryptor = create_wxchannels_decryptor;
globalThis.decrypt_wxchannels_data = decrypt_wxchannels_data;
