# toolprobe.rclabs.in

Static site that describes the toolprobe CLI. It is assets only.

It does not:

- call Workers AI, OpenAI, Anthropic, or Gemini
- store API keys
- create KV, R2, or containers
- publish a `workers.dev` hostname (`workers_dev` is false)

Deploy is manual, from an account that already controls the `rclabs.in` zone. Do not run this from CI.

## Deploy

From this directory, with Wrangler logged into that account:

```bash
npx wrangler deploy
```

Wrangler creates the custom domain `toolprobe.rclabs.in` on the zone (Cloudflare adds the DNS record and certificate). If a CNAME already exists for that hostname, remove it first, then deploy again.

Preview locally without deploying:

```bash
npx wrangler dev
```

To remove the hostname later, delete the custom domain in Workers → toolprobe → Settings → Domains & Routes, then delete the leftover certificate under SSL/TLS → Edge Certificates if Cloudflare created one.
