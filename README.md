# Spocker

Spocker is a lightweight container runtime tool that provides basic containerization features, allowing you to run processes within a sandboxed environment, isolating them from the host system. Spocker supports limiting resources such as memory, CPU shares, and block I/O weight, as well as providing namespace isolation and basic networking features.

The tool is controlled through a command-line interface and accepts various flags to customize the container environment, including flags for setting memory limits, CPU shares, block I/O weight, cgroup and namespace names, namespace types, filesystem root, and network configuration.

Spocker requires root privileges to execute and leverages Linux kernel features such as cgroups, namespaces, and network namespaces to provide containerization.

## Installation

Spocker requires a Linux environment to run. To install Spocker, follow these steps:

1. Clone the repository:

   ```
   git clone https://github.com/elispeigel/spocker.git
   ```

2. Change to the Spocker directory:

   ```
   cd spocker
   ```

3. Build the Spocker binary:

   ```
   go build -o spocker
   ```

4. Move the Spocker binary to a directory in your `PATH`:

   ```
   sudo mv spocker /usr/local/bin/
   ```

## Usage

To use Spocker, run the following command with the appropriate flags:

```
spocker run [flags] <command> [args...]
```

For example, to run a simple container with a limited amount of memory:

```
spocker run --memory-limit 100000000 /bin/bash
```

For more usage examples and flag descriptions, refer to the [documentation](docs/USAGE.md).

## Support

For support or to report any issues, please open an issue on the [issue tracker](https://github.com/elispeigel/spocker/issues).

## Roadmap

- Improve error handling and user feedback
- Add support for more resource constraints
- Improve networking support and configuration options
- Enhance container management and monitoring capabilities

## Contributing

Contributions are welcome! Please refer to the [contributing guide](CONTRIBUTING.md) for more information on how to contribute to the project.

## Authors and Acknowledgment

- Eli Speigel - Initial work and maintainer

Special thanks to all the [contributors](https://github.com/elispeigel/spocker/graphs/contributors) who have helped improve Spocker!

## License

Spocker is licensed under the [MIT License](LICENSE).

## Project Status

Spocker is under active development, and new features and improvements are planned. If you have any suggestions or would like to contribute, please feel free to open an issue or submit a pull request.