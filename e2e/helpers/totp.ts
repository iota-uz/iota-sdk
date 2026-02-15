import { createHmac } from 'crypto';

const TOTP_PERIOD_SECONDS = 30;
const TOTP_DIGITS = 6;
const BASE32_ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';

/**
 * Extract TOTP secret from otpauth:// URL
 */
export function extractSecretFromOTPAuthURL(otpauthURL: string): string {
	try {
		const url = new URL(otpauthURL);
		const secret = url.searchParams.get('secret');
		if (!secret) {
			throw new Error('no secret found in otpauth URL');
		}
		return secret;
	} catch (error) {
		throw new Error(`Failed to extract secret from OTP auth URL: ${error}`);
	}
}

/**
 * Generate a valid 6-digit TOTP code (RFC 6238 / SHA1 / 30s period).
 */
export function generateTOTPCode(secret: string): string {
	try {
		const step = Math.floor(Date.now() / 1000 / TOTP_PERIOD_SECONDS);
		return generateCodeAtStep(secret, step);
	} catch (error) {
		throw new Error(`Failed to generate TOTP code: ${error}`);
	}
}

/**
 * Generate an invalid TOTP code (for error testing).
 */
export function generateInvalidTOTPCode(secret: string): string {
	const validCode = generateTOTPCode(secret);

	for (let i = validCode.length - 1; i >= 0; i--) {
		const digit = Number.parseInt(validCode[i], 10);
		const nextDigit = ((digit + 1) % 10).toString();
		const candidate = `${validCode.slice(0, i)}${nextDigit}${validCode.slice(i + 1)}`;
		if (!verifyTOTPCode(secret, candidate)) {
			return candidate;
		}
	}

	throw new Error('failed to generate invalid TOTP code');
}

/**
 * Verify TOTP code with +/- one-step tolerance to mirror server-side skew.
 */
export function verifyTOTPCode(secret: string, code: string): boolean {
	if (!/^\d{6}$/.test(code)) {
		return false;
	}

	const step = Math.floor(Date.now() / 1000 / TOTP_PERIOD_SECONDS);
	for (let offset = -1; offset <= 1; offset++) {
		if (generateCodeAtStep(secret, step + offset) === code) {
			return true;
		}
	}

	return false;
}

function generateCodeAtStep(secret: string, step: number): string {
	const key = decodeBase32(secret);
	const message = Buffer.alloc(8);
	message.writeBigUInt64BE(BigInt(step));

	const digest = createHmac('sha1', key).update(message).digest();
	const offset = digest[digest.length - 1] & 0x0f;
	const binary =
		((digest[offset] & 0x7f) << 24) |
		((digest[offset + 1] & 0xff) << 16) |
		((digest[offset + 2] & 0xff) << 8) |
		(digest[offset + 3] & 0xff);

	return (binary % 10 ** TOTP_DIGITS).toString().padStart(TOTP_DIGITS, '0');
}

function decodeBase32(input: string): Buffer {
	const normalized = input.toUpperCase().replace(/[\s-]/g, '').replace(/=+$/g, '');
	if (!normalized) {
		throw new Error('secret is empty');
	}

	let bits = 0;
	let value = 0;
	const bytes: number[] = [];

	for (const char of normalized) {
		const index = BASE32_ALPHABET.indexOf(char);
		if (index === -1) {
			throw new Error(`invalid base32 character: ${char}`);
		}

		value = (value << 5) | index;
		bits += 5;

		while (bits >= 8) {
			bytes.push((value >>> (bits - 8)) & 0xff);
			bits -= 8;
		}
	}

	return Buffer.from(bytes);
}
