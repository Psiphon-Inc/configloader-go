package main

func Example_main() {
	main()

	// Output:
	// map[CORS:map[app_user_agents:[UA-1 UA-2]] Log:map[Level:debug] Stats:map[SampleCount:1000]]
	// {Nonsecret:{ 'CORS.app_user_agents':'config_nonsecret.toml'; 'Log.Level':'config_nonsecret_override.toml'; 'Stats.SampleCount':'[default]' } Secret:{ 'DB.Password':'config_secret.toml' }}
}
