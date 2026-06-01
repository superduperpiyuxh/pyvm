Register-ArgumentCompleter -Native -CommandName pyvm -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $commands = "install", "use", "list", "ls", "delete", "remove", "rm", "uninstall", "color", "theme", "which", "help"
    $commands | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
