// Name of the backend's HttpOnly session cookie. Server components only
// check whether it is present (a routing hint); its signature is verified
// exclusively by the Go API. Browser code never reads it.
export const SESSION_COOKIE = "bia_session";
