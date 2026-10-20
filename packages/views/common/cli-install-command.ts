/**
 * The command that installs the Multica CLI, run from a Multica checkout.
 *
 * The runtimes "add computer" dialog and onboarding's CLI card both show it,
 * so they read it from here instead of each hardcoding the string.
 */
export const CLI_INSTALL_COMMAND = "make local-release";
