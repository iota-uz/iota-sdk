import { test, expect, APIResponse, APIRequestContext } from '@playwright/test';

const baseURL = process.env.BASE_URL ?? 'http://localhost:3201';
// Prefer canonical OIDC_ISSUERURL; fall back to legacy OIDC_ISSUER_URL for operators still on old env names.
const configuredIssuer = process.env.OIDC_ISSUERURL ?? process.env.OIDC_ISSUER_URL ?? `${baseURL}/oidc`;

function assertOIDCEnabled(response: APIResponse, endpoint: string): void {
	expect(response.status(), `${endpoint} returned 404. Set OIDC_ISSUER_URL and OIDC_CRYPTO_KEY to enable OIDC.`).not.toBe(404);
}

async function getDiscoveryMetadata(request: APIRequestContext) {
	const response = await request.get('/.well-known/openid-configuration', {
		failOnStatusCode: false,
	});
	assertOIDCEnabled(response, '/.well-known/openid-configuration');
	expect(response.ok()).toBeTruthy();
	expect(response.headers()['content-type']).toContain('application/json');
	return response.json();
}

function endpointPath(endpointURL: string): string {
	return new URL(endpointURL).pathname;
}

test.describe('OIDC provider contracts', () => {
	test('serves discovery metadata with consistent endpoint URLs', async ({ request }) => {
		const metadata = await getDiscoveryMetadata(request);
		expect(metadata.issuer).toBe(configuredIssuer);
		expect(metadata.authorization_endpoint).toBe(`${configuredIssuer}/authorize`);
		expect(metadata.token_endpoint).toBe(`${configuredIssuer}/oauth/token`);
		expect(metadata.userinfo_endpoint).toBe(`${configuredIssuer}/userinfo`);
		expect(metadata.jwks_uri).toBe(`${configuredIssuer}/keys`);
		expect(metadata.response_types_supported).toContain('code');
		expect(metadata.grant_types_supported).toContain('authorization_code');
		expect(metadata.code_challenge_methods_supported).toContain('S256');
	});

	test('serves JWKS with only public RSA key material', async ({ request }) => {
		const metadata = await getDiscoveryMetadata(request);
		const response = await request.get(endpointPath(metadata.jwks_uri), {
			failOnStatusCode: false,
		});
		assertOIDCEnabled(response, endpointPath(metadata.jwks_uri));
		expect(response.ok()).toBeTruthy();
		expect(response.headers()['content-type']).toContain('application/json');

		const jwks = await response.json();
		expect(Array.isArray(jwks.keys)).toBeTruthy();
		expect(jwks.keys.length).toBeGreaterThan(0);

		for (const key of jwks.keys) {
			expect(key.kty).toBe('RSA');
			expect(key.use).toBe('sig');
			expect(typeof key.kid).toBe('string');
			expect(key.kid.length).toBeGreaterThan(0);
			expect(typeof key.n).toBe('string');
			expect(typeof key.e).toBe('string');
			expect(key.d).toBeUndefined();
		}
	});

	test('rejects authorize requests without required oauth parameters', async ({ request }) => {
		const metadata = await getDiscoveryMetadata(request);
		const authorizePath = endpointPath(metadata.authorization_endpoint);
		const response = await request.get(authorizePath, {
			failOnStatusCode: false,
		});
		assertOIDCEnabled(response, authorizePath);
		expect(response.status()).toBe(400);

		const body = await response.text();
		expect(body.toLowerCase()).toContain('client_id');
	});

	test('rejects unsupported token grant types with OAuth error payload', async ({ request }) => {
		const metadata = await getDiscoveryMetadata(request);
		const tokenPath = endpointPath(metadata.token_endpoint);
		const response = await request.post(tokenPath, {
			form: {
				grant_type: 'client_credentials',
				client_id: 'invalid-client',
				client_secret: 'invalid-secret',
				scope: 'openid',
			},
			failOnStatusCode: false,
		});
		assertOIDCEnabled(response, tokenPath);
		expect(response.status()).toBe(400);
		expect(response.headers()['content-type']).toContain('application/json');

		const payload = await response.json();
		expect(payload.error).toBe('unsupported_grant_type');
		expect(payload.error_description).toContain('not supported');
	});

	test('rejects userinfo calls without bearer token', async ({ request }) => {
		const metadata = await getDiscoveryMetadata(request);
		const userInfoPath = endpointPath(metadata.userinfo_endpoint);
		const response = await request.get(userInfoPath, {
			failOnStatusCode: false,
		});
		assertOIDCEnabled(response, userInfoPath);
		expect(response.status()).toBe(401);

		const body = await response.text();
		expect(body.toLowerCase()).toContain('access token invalid');
	});
});
