package cmd

flags := rootCmd.PersistentFlags()
	if getBoolP(flags, "version") {
		fmt.Println("maddox v" + version)
		return nil
	}