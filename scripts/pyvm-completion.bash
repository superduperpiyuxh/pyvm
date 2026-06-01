_pyvm_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local commands="install use list ls delete remove rm uninstall color theme which help"
    COMPREPLY=( $(compgen -W "${commands}" -- ${cur}) )
}
complete -F _pyvm_completions pyvm
