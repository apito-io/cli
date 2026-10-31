package main

func init() {
	initLoginFlags()
	initKeyFlags()
	initMCPFlags()
	initGenFlags(genCmd)
	initGenFlags(typesCmd)
	initTenantFlags()
	initCloudFlags()

	telemetryCmd.AddCommand(telemetryEnableCmd, telemetryDisableCmd, telemetryStatusCmd)
	rootCmd.AddCommand(telemetryCmd)

	rootCmd.AddCommand(loginCmd)

	projectCmd.AddCommand(projectListCmd, projectOpenCmd)
	rootCmd.AddCommand(projectCmd)

	keyProjectCmd.AddCommand(keyProjectListCmd, keyProjectCreateCmd, keyProjectRevokeCmd)
	keyCmd.AddCommand(keyProjectCmd)
	rootCmd.AddCommand(keyCmd)

	mcpCmd.AddCommand(mcpConnectCmd, mcpConfigCmd, mcpServeCmd)
	rootCmd.AddCommand(mcpCmd)

	rootCmd.AddCommand(genCmd, typesCmd)

	tenantCmd.AddCommand(tenantListCmd, tenantCreateCmd)
	rootCmd.AddCommand(tenantCmd)

	clientCmd.AddCommand(clientInviteCmd, clientStatusCmd)
	rootCmd.AddCommand(clientCmd)

	backupCmd.AddCommand(backupListCmd, backupRunCmd)
	rootCmd.AddCommand(backupCmd)

	restoreCmd.AddCommand(restorePreviewCmd, restoreApplyCmd)
	rootCmd.AddCommand(restoreCmd)
}
