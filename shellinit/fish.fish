# awsdo history: Up/Down step through awsdo history while the line starts with
# "awsdo ". Ctrl-X h does the same as Up. Elsewhere the keys keep their
# normal behavior.
set -g __awsdo_entries
set -g __awsdo_pos 0
set -g __awsdo_shown ""
set -g __awsdo_draft ""

function __awsdo_step
    set -l direction $argv[1]
    set -l buffer (commandline)
    if test -z "$__awsdo_shown"; or test "$buffer" != "$__awsdo_shown"
        string match -q 'awsdo *' -- $buffer; or return 1
        set -g __awsdo_entries (awsdo history --raw 2>/dev/null)
        set -g __awsdo_pos (math (count $__awsdo_entries) + 1)
        set -g __awsdo_draft $buffer
    end

    set -l total (count $__awsdo_entries)
    set -l new (math $__awsdo_pos + $direction)
    test $new -lt 1; and set new 1
    if test $new -gt $total
        set -g __awsdo_pos (math $total + 1)
        set -g __awsdo_shown $__awsdo_draft
    else
        set -g __awsdo_pos $new
        set -g __awsdo_shown "awsdo $__awsdo_entries[$new]"
    end
    commandline --replace -- $__awsdo_shown
    commandline --cursor (string length -- $__awsdo_shown)
end

function __awsdo_up
    __awsdo_step -1; or up-or-search
end

function __awsdo_down
    __awsdo_step 1; or down-or-search
end

function __awsdo_reset --on-event fish_prompt
    set -g __awsdo_shown ""
end

bind up __awsdo_up
bind down __awsdo_down
bind \cxh __awsdo_up
