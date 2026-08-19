/**
 * Protects the Pages site with the same Basic credentials used by the Hub Worker.
 *
 * @param {Request} request
 * @param {{ HUB_ADMIN_TOKEN?: string }} env
 * @returns {boolean}
 */
function admin_authorized(request, env) {
  const admin_token = typeof env.HUB_ADMIN_TOKEN === "string" ? env.HUB_ADMIN_TOKEN : "";
  const authorization = request.headers.get("Authorization") || "";
  if (admin_token === "" || !authorization.startsWith("Basic ")) {
    return false;
  }
  try {
    const credentials = atob(authorization.slice(6));
    const separator = credentials.indexOf(":");
    if (separator < 0 || credentials.slice(0, separator) !== "admin") {
      return false;
    }
    return safe_equal(credentials.slice(separator + 1), admin_token);
  } catch {
    return false;
  }
}

/**
 * @param {string} left
 * @param {string} right
 * @returns {boolean}
 */
function safe_equal(left, right) {
  const encoder = new TextEncoder();
  const left_bytes = encoder.encode(left);
  const right_bytes = encoder.encode(right);
  const length = Math.max(left_bytes.length, right_bytes.length);
  let difference = left_bytes.length ^ right_bytes.length;
  for (let index = 0; index < length; index += 1) {
    difference |= (left_bytes[index] || 0) ^ (right_bytes[index] || 0);
  }
  return difference === 0;
}

/** @returns {Response} */
function authorization_required() {
  return new Response("Authorization required", {
    status: 401,
    headers: {
      "Cache-Control": "no-store",
      "Content-Type": "text/plain; charset=utf-8",
      "WWW-Authenticate": 'Basic realm="WX Channels Hub Admin", charset="UTF-8"',
    },
  });
}

/**
 * @param {{ request: Request, env: { HUB_ADMIN_TOKEN?: string }, next: () => Promise<Response> }} context
 * @returns {Promise<Response>}
 */
export async function onRequest(context) {
  if (!admin_authorized(context.request, context.env)) {
    return authorization_required();
  }

  const response = await context.next();
  const headers = new Headers(response.headers);
  headers.set("Content-Security-Policy", "default-src 'none'; connect-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'");
  headers.set("Referrer-Policy", "no-referrer");
  headers.set("X-Content-Type-Options", "nosniff");
  headers.set("X-Frame-Options", "DENY");
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
}

