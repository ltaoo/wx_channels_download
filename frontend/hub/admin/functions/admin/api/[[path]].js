/**
 * Proxies the same-origin Pages API route to the Durable Objects Worker.
 *
 * @param {{ request: Request, env: { HUB: Fetcher } }} context
 * @returns {Promise<Response>}
 */
export async function onRequest(context) {
  return context.env.HUB.fetch(context.request);
}

