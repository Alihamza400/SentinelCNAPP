package config

import ("testing")

func TestGet(t *testing.T) {
	c := NewWithMap(map[string]string{
		"SENTINEL_POSTGRES_URL": "test-url",
	}) 
	
	value, ok := c.Get("SENTINEL_POSTGRES_URL")
	if !ok {
		t.Fatal("Expected env var to exist")
	}
	
	if value != "test-url" {
		t.Errorf("Expected %s, got %s", "test-url", value)
	}
}

func TestGetDefault(t *testing.T) {
	c := NewWithMap(map[string]string{})
	
	defaultVal := "default-url"
	value := c.GetDefault("SENTINEL_POSTGRES_URL", defaultVal)
	
	if value != defaultVal {
		t.Errorf("Expected default %s, got %s", defaultVal, value)
	}
}

func TestGetInt(t *testing.T) {
	c := NewWithMap(map[string]string{
		"SENTINEL_SOME_INT": "123"
	}) 
	
	value := c.GetInt("SENTINEL_SOME_INT", 0)
	
	if value != 123 {
		t.Errorf("Expected %d, got %d", 123, value)
	}
}

func TestGetBool(t *testing.T) {
	c := NewWithMap(map[string]string{
		"SENTINEL_ENABLE_FEATURE": "true"
	}) 
	
	value := c.GetBool("SENTINEL_ENABLE_FEATURE", false)
	
	if value != true {
		t.Errorf("Expected %t, got %t", true, value)
	}
}

func TestGetStringSlice(t *testing.T) {
	c := NewWithMap(map[string]string{
		"SENTINEL_TAGS": "a,b,c"
	}) 
	
	values := c.GetStringSlice("SENTINEL_TAGS", []string{})
	
	if len(values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(values))
	}
	
	if !contains(values, "a") || !contains(values, "b") || !contains(values, "c") {
		t.Errorf("Expected values [a,b,c], got %v", values)
	}
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
