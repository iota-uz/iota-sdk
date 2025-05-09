// CORS configuration for API requests to ngrok
export const fetchOptions = {
  headers: {
    'Content-Type': 'application/json',
    // Add any additional headers required by your API
  },
  // Include credentials if your API requires authentication
  credentials: 'include' as RequestCredentials,
  // Enable CORS mode
  mode: 'cors' as RequestMode,
};
