export const PASSWORD_POLICY_MESSAGE = 'Use at least 15 characters and no more than 72 UTF-8 bytes.';

export const passwordByteLength = password => new TextEncoder().encode(password).length;

export const isNewPasswordValid = password => [...password].length >= 15 && passwordByteLength(password) <= 72;
