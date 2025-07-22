## To be able to use the docker-pussh as a script and not as a pluging while still having completion
function _docker-pussh_image_completions
    printf "%s\n" (docker image ls --format '{{.Repository}}:{{.Tag}}')
end
function _docker-pussh_ssh_host_completions
    set -l ssh_config_files ~/.ssh/config ~/.ssh/config.d/*
    set -l ssh_known_hosts ~/.ssh/known_hosts

    printf "%s\n" (awk '/^Host / {print $2}' $ssh_config_files 2>/dev/null) (awk '{ gsub(/\[|\]/,""); print $1 }' $ssh_known_hosts 2>/dev/null)
end

complete -c "docker-pussh" -n '__fish_is_nth_token 1' -fa '(_docker-pussh_image_completions)'
complete -c "docker-pussh" -n '__fish_is_nth_token 2' -fa '(_docker-pussh_ssh_host_completions)'
