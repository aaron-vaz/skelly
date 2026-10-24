package autocomplete

// Each script asks 'skelly autocomplete complete' for the candidates of the words typed so
// far, so they only differ in how a shell wants to be given them. Candidates come one per
// line as a value, a tab and a description, and a last line ":directive" says whether the
// files or the directories of the current path are to be offered too.

const bashScript = `# bash completion for skelly. Add this to ~/.bashrc:
#   eval "$(skelly autocomplete bash)"
_skelly_complete() {
    local cur line directive=none flag out
    cur="${COMP_WORDS[COMP_CWORD]}"
    COMPREPLY=()

    out=$(skelly autocomplete complete -- "${COMP_WORDS[@]:1:COMP_CWORD}" 2>/dev/null) || return 0
    while IFS= read -r line; do
        case "$line" in
            :*) directive="${line#:}" ;;
            "") ;;
            *) COMPREPLY+=("${line%%$'\t'*}") ;;
        esac
    done <<< "$out"

    if [[ $directive == files || $directive == dirs ]]; then
        flag=-f
        [[ $directive == dirs ]] && flag=-d
        while IFS= read -r line; do
            COMPREPLY+=("$line")
        done < <(compgen $flag -- "$cur")
        # Directories get a slash and names with spaces are escaped
        type compopt >/dev/null 2>&1 && compopt -o filenames
    fi
    return 0
}
complete -F _skelly_complete skelly
`

const zshScript = `# zsh completion for skelly. Add this to ~/.zshrc:
#   eval "$(skelly autocomplete zsh)"
_skelly() {
    local line value desc directive=none
    local -a out candidates
    out=("${(@f)$(skelly autocomplete complete -- "${(@)words[2,CURRENT]}" 2>/dev/null)}")
    for line in "${out[@]}"; do
        case $line in
            :*) directive=${line#:} ;;
            '') ;;
            *)
                value=${line%%$'\t'*}
                desc=''
                [[ $line == *$'\t'* ]] && desc=${line#*$'\t'}
                # _describe reads name:description, so a colon in a name is escaped
                value=${value//:/\\:}
                if [[ -n $desc ]]; then
                    candidates+=("$value:$desc")
                else
                    candidates+=("$value")
                fi
                ;;
        esac
    done

    (( $#candidates )) && _describe -t skelly 'skelly' candidates
    case $directive in
        files) _files ;;
        dirs) _files -/ ;;
    esac
}
if (( $+functions[compdef] )); then
    compdef _skelly skelly
else
    # compinit has not run yet, so run it for the completion to work
    autoload -Uz compinit && compinit && compdef _skelly skelly
fi
`

const fishScript = `# fish completion for skelly. Add this to ~/.config/fish/config.fish:
#   skelly autocomplete fish | source
function __skelly_complete
    set -l cur (commandline -ct)
    set -l words (commandline -opc)
    set -e words[1]
    set -l directive none
    for line in (skelly autocomplete complete -- $words "$cur" 2>/dev/null)
        if string match -q -- ':*' $line
            set directive (string sub -s 2 -- $line)
        else if test -n "$line"
            echo $line
        end
    end
    switch $directive
        case files
            __fish_complete_path "$cur"
        case dirs
            __fish_complete_directories "$cur"
    end
end
complete -c skelly -f -a '(__skelly_complete)'
`
