# Distribution
# This file builds the distribution packages
# and install the application.

##############
### Common ###
##############

def get_pkg_type()
    # Read environment variable
    if !ENV["PKG_TYPE"].nil?
        return ENV["PKG_TYPE"]
    end

    # Mapping between the package type and a command to check if the type is supported.
    supported_type_checks = [
        ["rpm", ["rpm", "-q", "-a"]],
        ["deb", ["dpkg", "-l"]],
        ["apk", ["apk", "--version"]],
        ["freebsd", ["freebsd-version"]],
    ]

    supported_types = []

    supported_type_checks.each do |type, check|
        begin
            if check[0].kind_of? Array
                statuses = Open3.pipeline(*check)
                status = statuses[-1]
            else
                _, _, status = Open3.capture3(*check)
            end

            if status.success?
                supported_types.append(type)
            end
        rescue Exception
            # Command not exist
        end
    end

    if supported_types.empty?
        fail "Unknown package type for current OS."
    elsif supported_types.length != 1
        fail "Ambiguous package type for current OS: #{supported_types}. Use PKG_TYPE to specify one of them"
    end

    return supported_types[0]
end

pkgs_dir = "dist/pkgs"
directory pkgs_dir

default_os_binary_directory = "/usr/bin"
if OS == "OpenBSD" || OS == "FreeBSD"
    default_os_binary_directory = "/usr/local/bin"
end

CLEAN.append "dist"

sed_regex_to_expand_the_relative_path_to_executable_in_systemd_service_file =
    's/'+ # Regex begin
    '\(ExecStart=\)'+ # Pattern to match - the parameter name.
                      # The parameter name is saved in the first capture group.
    '/'+ # Delimiter.
    # Replacement:
    '\1'+ # Copy the parameter name.
    default_os_binary_directory.gsub('/', '\/')+ # Append the escaped absolute binary
                                                 # directory before the relative path.
    '\/'+ # Escaped path delimiter.
    '/g' # Regex end

##############
### Agent ###
##############

agent_dist_bin_dir = File.join("dist/agent", default_os_binary_directory)
directory agent_dist_bin_dir
agent_dist_bin_file = File.join(agent_dist_bin_dir, "stork-agent")
file agent_dist_bin_file => [agent_dist_bin_dir, AGENT_BINARY_FILE] do
    sh "cp", "-a", AGENT_BINARY_FILE, agent_dist_bin_file
end

agent_dist_man_dir = "dist/agent/usr/share/man/man8"
directory agent_dist_man_dir
agent_dist_man_file = File.join(agent_dist_man_dir, "stork-agent.8")
file agent_dist_man_file => [agent_dist_man_dir, AGENT_MAN_FILE] do
    sh "cp", "-a", AGENT_MAN_FILE, agent_dist_man_file
end

agent_dist_system_dir = "dist/agent/lib/systemd/system/"
directory agent_dist_system_dir
agent_dist_system_service_file = File.join(agent_dist_system_dir, "isc-stork-agent.service")
file agent_dist_system_service_file => [SED, agent_dist_system_dir, "etc/isc-stork-agent.service"] do
    sh "cp", "-a", "etc/isc-stork-agent.service", agent_dist_system_service_file
    sh "chmod", "644", agent_dist_system_service_file
    # Use the abolute path to the executable
    sh SED,
        "-i.tmp", # Modify in place
        sed_regex_to_expand_the_relative_path_to_executable_in_systemd_service_file, # Regex
        agent_dist_system_service_file # File to modify
    # Remove the backup file created by sed.
    sh "rm", "-f", agent_dist_system_service_file + ".tmp"
end

agent_etc_files = FileList["etc/agent.env", "etc/agent-credentials.json.template"]
agent_dist_etc_dir = "dist/agent/etc/stork"
file agent_dist_etc_dir => agent_etc_files do
    sh "mkdir", "-p", agent_dist_etc_dir
    agent_etc_files.each do |file|
        FileUtils.cp(file, agent_dist_etc_dir)
    end
    sh "touch", "-c", agent_dist_etc_dir
end

agent_dist_dir = "dist/agent"
directory agent_dist_dir
file agent_dist_dir => [agent_dist_bin_file, agent_dist_man_file, agent_dist_system_service_file, agent_dist_etc_dir]

agent_hooks = FileList["etc/hooks/**/isc-stork-agent.post*", "etc/hooks/**/isc-stork-agent.pre*"]

AGENT_PACKAGE_STUB_FILE = File.join(pkgs_dir, "agent-built.pkg")
file AGENT_PACKAGE_STUB_FILE => [FPM, MAKE, GCC, agent_dist_dir, pkgs_dir] + agent_hooks do
    ENV["PKG_NAME"] = "agent"
    Rake::Task["clean:pkgs"].invoke()

    version = `#{AGENT_BINARY_FILE} --version`.rstrip
    pkg_type = get_pkg_type()

    agent_dist_dir_abs = File.expand_path(agent_dist_dir)

    Dir.chdir(pkgs_dir) do
        stdout, stderr, status = Open3.capture3 FPM,
            "-C", agent_dist_dir_abs,
            "-n", "isc-stork-agent",
            "-s", "dir",
            "-t", pkg_type,
            "-v", "#{version}.#{TIMESTAMP}",
            "--after-install", "../../etc/hooks/#{pkg_type}/isc-stork-agent.postinst",
            "--after-remove", "../../etc/hooks/#{pkg_type}/isc-stork-agent.postrm",
            "--before-remove", "../../etc/hooks/#{pkg_type}/isc-stork-agent.prerm",
            "--config-files", "etc/stork/agent.env",
            "--config-files", "etc/stork/agent-credentials.json.template",
            "--description", "ISC Stork Agent",
            "--license", "MPL 2.0",
            "--url", "https://gitlab.isc.org/isc-projects/stork/",
            "--vendor", "Internet Systems Consortium, Inc."
        if status != 0
            puts status, stdout, stderr
            fail
        end
    end
    sh "touch", AGENT_PACKAGE_STUB_FILE
end

##############
### Server ###
##############

server_dist_bin_dir = File.join("dist/server", default_os_binary_directory)
directory server_dist_bin_dir
server_dist_bin_file = File.join(server_dist_bin_dir, "stork-server")
file server_dist_bin_file => [server_dist_bin_dir, SERVER_BINARY_FILE] do
    sh "cp", "-a", SERVER_BINARY_FILE, server_dist_bin_file
end
tool_dist_bin_file = File.join(server_dist_bin_dir, "stork-tool")
file tool_dist_bin_file => [server_dist_bin_dir, TOOL_BINARY_FILE] do
    sh "cp", "-a", TOOL_BINARY_FILE, tool_dist_bin_file
end

server_dist_man_dir = "dist/server/usr/share/man/man8"
directory server_dist_man_dir
server_dist_man_file = File.join(server_dist_man_dir, "stork-server.8")
file server_dist_man_file => [server_dist_man_dir, SERVER_MAN_FILE] do
    sh "cp", "-a", SERVER_MAN_FILE, server_dist_man_file
end
tool_dist_man_file = File.join(server_dist_man_dir, "stork-tool.8")
file tool_dist_man_file => [server_dist_man_dir, TOOL_MAN_FILE] do
    sh "cp", "-a", TOOL_MAN_FILE, tool_dist_man_file
end

server_dist_system_dir = "dist/server/lib/systemd/system/"
directory server_dist_system_dir
server_dist_system_service_file = File.join(server_dist_system_dir, "isc-stork-server.service")
file server_dist_system_service_file => [SED, server_dist_system_dir, "etc/isc-stork-server.service"] do
    sh "cp", "-a", "etc/isc-stork-server.service", server_dist_system_service_file
    sh "chmod", "644", server_dist_system_service_file
    # Use the abolute path to the executable
    sh SED,
        "-i.tmp", # Modify in place
        sed_regex_to_expand_the_relative_path_to_executable_in_systemd_service_file, # Regex
        server_dist_system_service_file # File to modify
    # Remove the backup file created by sed.
    sh "rm", "-f", server_dist_system_service_file + ".tmp"
end

server_etc_files = FileList["etc/server.env"]
server_dist_etc_dir = "dist/server/etc/stork"
file server_dist_etc_dir => server_etc_files do
    sh "mkdir", "-p", server_dist_etc_dir
    server_etc_files.each do |file|
        FileUtils.cp(file, server_dist_etc_dir)
    end
    sh "touch", "-c", server_dist_etc_dir
end

server_examples_dir = "dist/server/usr/share/stork/examples"
directory server_examples_dir

server_grafana_examples_dir = File.join(server_examples_dir, "grafana")
file server_grafana_examples_dir => FileList["grafana/*.json"] do
    sh "mkdir", "-p", server_grafana_examples_dir
    sh "cp", "-a", *FileList["grafana/*.json"], server_grafana_examples_dir
    sh "touch", "-c", server_grafana_examples_dir
end

server_nginx_example_file = File.join(server_examples_dir, "nginx-stork.conf")
file server_nginx_example_file => ["etc/nginx-stork.conf", server_examples_dir] do
    sh "cp", "-a", "etc/nginx-stork.conf", server_examples_dir
end

server_www_dir = "dist/server/usr/share/stork/www"
file server_www_dir => [WEBUI_DIST_DIRECTORY, WEBUI_DIST_ARM_DIRECTORY] do
    sh "mkdir", "-p", server_www_dir
    sh "cp", "-a", *FileList[File.join(WEBUI_DIST_DIRECTORY, "*")], server_www_dir
    sh "touch", "-c", server_www_dir
end

server_dist_dir_tool_part = [tool_dist_bin_file]
server_dist_dir_man_part = [tool_dist_man_file, server_dist_man_file]
server_dist_dir_server_part = [server_dist_bin_file, server_dist_system_service_file, server_dist_etc_dir]
server_dist_dir_webui_part = [server_nginx_example_file, server_grafana_examples_dir, server_www_dir]

server_dist_dir = "dist/server"
directory server_dist_dir
file server_dist_dir => server_dist_dir_tool_part + server_dist_dir_man_part + server_dist_dir_server_part + server_dist_dir_webui_part

server_hooks = FileList["etc/hooks/**/isc-stork-server.post*", "etc/hooks/**/isc-stork-server.pre*"]

SERVER_PACKAGE_STUB_FILE = File.join(pkgs_dir, "server-built.pkg")
file SERVER_PACKAGE_STUB_FILE => [FPM, MAKE, GCC, server_dist_dir, pkgs_dir] + server_hooks do
    ENV["PKG_NAME"] = "server"
    Rake::Task["clean:pkgs"].invoke()

    version = `#{SERVER_BINARY_FILE} --version`.rstrip
    pkg_type = get_pkg_type()

    server_dist_dir_abs = File.expand_path(server_dist_dir)

    Dir.chdir(pkgs_dir) do
        sh FPM,
            "-C", server_dist_dir_abs,
            "-n", "isc-stork-server",
            "-s", "dir",
            "-t", pkg_type,
            "-v", "#{version}.#{TIMESTAMP}",
            "--after-install", "../../etc/hooks/#{pkg_type}/isc-stork-server.postinst",
            "--after-remove", "../../etc/hooks/#{pkg_type}/isc-stork-server.postrm",
            "--before-remove", "../../etc/hooks/#{pkg_type}/isc-stork-server.prerm",
            "--config-files", "etc/stork/server.env",
            "--description", "ISC Stork Server",
            "--license", "MPL 2.0",
            "--url", "https://gitlab.isc.org/isc-projects/stork/",
            "--vendor", "Internet Systems Consortium, Inc."
    end
    sh "touch", SERVER_PACKAGE_STUB_FILE
end

#############
### Tasks ###
#############

namespace :clean do
    desc "Clean all packages of a given kind (agent or server)
        PKG_NAME - package name - choice: 'agent' or 'server', optional
    "
    task :pkgs do
        pkgs = FileList[File.join(pkgs_dir, "isc-stork-#{ENV["PKG_NAME"]}*")]
        stub = "-built.pkg"
        if !ENV["PKG_NAME"].nil?
            stub = ENV["PKG_NAME"] + stub
        else
            stub = "*" + stub
        end
        stubs = FileList[File.join(pkgs_dir, stub)]
        files = pkgs + stubs
        if !files.empty?
            sh "rm", "-f", *files
        end
    end
end

namespace :utils do
    desc "Check package type of current OS"
    task :print_pkg_type do
        puts get_pkg_type()
    end

    desc "Remove the suffix from the last created package"
    task :remove_last_package_suffix do
        packages = Dir.glob(
            File.join(pkgs_dir, "isc-stork-*")
        ).sort_by {|filename| File.mtime(filename) }

        if packages.empty?
            fail "Missing packages"
        end

        last_package = packages[-1]

        prefix = last_package[/^.*isc-stork-[a-z]+/]
        _, dot, extension = last_package.rpartition(".")
        renamed_package = prefix + dot + extension
        File.rename(last_package, renamed_package)
    end
end

namespace :build do
    desc "Build agent package"
    task :agent_pkg => [AGENT_PACKAGE_STUB_FILE]

    desc "Build agent distribution directory"
    task :agent_dist => [agent_dist_dir]

    desc "Build server package"
    task :server_pkg => [SERVER_PACKAGE_STUB_FILE]

    desc "Build server distribution directory"
    task :server_dist => [server_dist_dir]

    desc "Build server distribution directory without WebUI, doc and tool"
    task :server_only_dist => server_dist_dir_server_part

    desc "Build server distribution directory only with WebUI (without server, doc and tool)"
    task :ui_only_dist => server_dist_dir_webui_part

end

namespace :rebuild do
    desc "Rebuild agent package"
    task :agent_pkg do
        sh "rm", "-f", AGENT_PACKAGE_STUB_FILE
        Rake::Task["build:agent_pkg"].invoke()
    end

    desc "Rebuild server package"
    task :server_pkg do
        sh "rm", "-f", SERVER_PACKAGE_STUB_FILE
        Rake::Task["build:server_pkg"].invoke()
    end
end

namespace :install do
    desc "Install agent
        DEST - destination directory - default: /"
    task :agent => [agent_dist_dir] do
        if ENV["DEST"].nil?
            ENV["DEST"] = "/"
        end
        sh "mkdir", "-p", ENV["DEST"]
#        sh "cp", "-a", "-f", "-r", File.join(agent_dist_dir, "."), ENV["DEST"]
        sh "rsync", "-l", "-L", "--copy-unsafe-links", "--safe-links", "-k", "-K", "--ignore-existing", File.join(agent_dist_dir, "."), ENV["DEST"]
    end

    desc "Install server
        DEST - destination directory - default: /"
    task :server => [server_dist_dir] do
        if ENV["DEST"].nil?
            ENV["DEST"] = "/"
        end
        sh "mkdir", "-p", ENV["DEST"]
        sh "cp", "-a", "-f", File.join(server_dist_dir, "."), ENV["DEST"]
    end
end


namespace :prepare do
    desc 'Install the external dependencies related to the distribution'
    task :dist do
        find_and_prepare_deps(__FILE__)
    end
end

namespace :check do
    desc 'Check the external dependencies related to the distribution'
    task :dist do
        check_deps(__FILE__)
    end
end
