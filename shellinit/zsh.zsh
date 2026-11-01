# awsdo history: Up/Down step through awsdo history while the line starts with
# "awsdo ". Ctrl-X h does the same as Up. Elsewhere the keys keep their
# normal behavior.
typeset -ga _awsdo_entries
typeset -gi _awsdo_pos=0
typeset -g _awsdo_shown="" _awsdo_draft=""

_awsdo_step() {
  local direction=$1 new
  if [[ -z $_awsdo_shown || $BUFFER != "$_awsdo_shown" ]]; then
    [[ $BUFFER == "awsdo "* ]] || return 1
    _awsdo_entries=(${(f)"$(awsdo history --raw 2>/dev/null)"})
    _awsdo_pos=$(( ${#_awsdo_entries} + 1 ))
    _awsdo_draft=$BUFFER
  fi

  new=$(( _awsdo_pos + direction ))
  (( new < 1 )) && new=1
  if (( new > ${#_awsdo_entries} )); then
    _awsdo_pos=$(( ${#_awsdo_entries} + 1 ))
    BUFFER=$_awsdo_draft
  else
    _awsdo_pos=$new
    BUFFER="awsdo ${_awsdo_entries[new]}"
  fi
  CURSOR=${#BUFFER}
  _awsdo_shown=$BUFFER
}

_awsdo_up()   { _awsdo_step -1 || zle up-line-or-history }
_awsdo_down() { _awsdo_step 1 || zle down-line-or-history }
_awsdo_reset() { _awsdo_shown="" }

zle -N _awsdo_up
zle -N _awsdo_down
autoload -Uz add-zle-hook-widget
add-zle-hook-widget line-init _awsdo_reset

bindkey '^[[A' _awsdo_up
bindkey '^[OA' _awsdo_up
bindkey '^[[B' _awsdo_down
bindkey '^[OB' _awsdo_down
bindkey '^Xh' _awsdo_up
