# awsdo history: while the line starts with "awsdo ", Ctrl-X h steps to an
# older entry and Ctrl-X H to a newer one. Bash cannot rebind the Up arrow only
# for awsdo lines, so Up keeps its normal behavior.
_awsdo_entries=()
_awsdo_pos=0
_awsdo_shown=""
_awsdo_draft=""

_awsdo_step() {
  local direction=$1 line new
  if [[ -z $_awsdo_shown || $READLINE_LINE != "$_awsdo_shown" ]]; then
    [[ $READLINE_LINE == "awsdo "* ]] || return 0
    _awsdo_entries=()
    while IFS= read -r line; do
      _awsdo_entries+=("$line")
    done < <(awsdo history --raw 2>/dev/null)
    _awsdo_pos=${#_awsdo_entries[@]}
    _awsdo_draft=$READLINE_LINE
  fi

  new=$(( _awsdo_pos + direction ))
  (( new < 0 )) && new=0
  if (( new >= ${#_awsdo_entries[@]} )); then
    _awsdo_pos=${#_awsdo_entries[@]}
    READLINE_LINE=$_awsdo_draft
  else
    _awsdo_pos=$new
    READLINE_LINE="awsdo ${_awsdo_entries[new]}"
  fi
  READLINE_POINT=${#READLINE_LINE}
  _awsdo_shown=$READLINE_LINE
}

_awsdo_older() { _awsdo_step -1; }
_awsdo_newer() { _awsdo_step 1; }
_awsdo_reset() { _awsdo_shown=""; }

PROMPT_COMMAND="_awsdo_reset${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
bind -x '"\C-xh": _awsdo_older'
bind -x '"\C-xH": _awsdo_newer'
