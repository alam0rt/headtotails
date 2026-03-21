package translate

// PolicyToResponse wraps the raw policy string for the API response.
func PolicyToResponse(policy string) map[string]string {
	return map[string]string{"policy": policy}
}
