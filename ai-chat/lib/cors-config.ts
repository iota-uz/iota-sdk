// Import types from lib.dom.d.ts
type FetchCredentials = 'omit' | 'same-origin' | 'include';
type FetchMode = 'navigate' | 'same-origin' | 'no-cors' | 'cors';

// CORS configuration for API requests to ngrok
export const fetchOptions = {
  headers: {
    'Content-Type': 'application/json',
    // Add any additional headers required by your API
  },
  // Include credentials if your API requires authentication
  credentials: 'include' as FetchCredentials,
  // Enable CORS mode
  mode: 'cors' as FetchMode,
};
