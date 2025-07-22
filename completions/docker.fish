function _docker-pussh_image_completions
    set -l docker_image (__fish_nth_token 1)
    printf "%s\n" (docker image ls --format '{{.Repository}}:{{.Tag}}')
end
function _docker-pussh_ssh_host_completions
    set -l ssh_host (__fish_nth_token 2)
    set -l ssh_config_files ~/.ssh/config ~/.ssh/config.d/*
    set -l ssh_known_hosts ~/.ssh/known_hosts

    printf "%s\n" (awk '/^Host / {print $2}' $ssh_config_files 2>/dev/null) (awk '{ gsub(/\[|\]/,""); print $1 }' $ssh_known_hosts 2>/dev/null)
end

# token 2 as pussh is a subcommand and as such is considered as token nth 1
complete -c "docker" -n '__fish_is_nth_token 2' -n '__fish_seen_subcommand_from pussh' -fa '(_docker-pussh_image_completions)'
complete -c "docker" -n '__fish_is_nth_token 3' -n '__fish_seen_subcommand_from pussh' -fa '(_docker-pussh_ssh_host_completions)'
