# CONTRIBUTING

Welcome to the Spocker contribution guide! We're excited to have you here and grateful for your interest in contributing to our project. This document outlines the process for contributing to Spocker, and it should help you get started with your first contribution.

## Code of Conduct

By participating in this project, you are expected to uphold our [Code of Conduct](CODE_OF_CONDUCT.md). Please take a moment to read it before contributing.

## Getting Started

1. **Fork the repository**: To contribute to this project, start by forking the repository. This will create a copy of the project under your own GitHub account, which allows you to make changes and submit a pull request.

2. **Clone your fork**: After forking the repository, clone your fork to your local machine using the following command:

   ```
   git clone https://github.com/elispeigel/spocker.git
   ```

3. **Set up your development environment**: Follow the instructions in the [README.md](README.md) file to set up your development environment.

## How to Contribute

1. **Choose an issue**: Look for open issues in the [issues tab](https://github.com/elispeigel/spocker/issues) and choose one that you would like to work on. Leave a comment on the issue to let others know you're working on it.

2. **Create a new branch**: Create a new branch with a descriptive name related to the issue you're working on:

   ```
   git checkout -b feature/your-feature-name
   ```

3. **Make your changes**: Work on the issue, and when you're ready, commit your changes. Write clear and concise commit messages that explain the changes you've made.

4. **Sync your fork**: Before submitting your changes, make sure your fork is up to date with the main branch of the project:

   ```
   git remote add upstream https://github.com/elispeigel/spocker.git
   git fetch upstream
   git merge upstream/main
   ```

5. **Push your changes**: Push your changes to your fork:

   ```
   git push origin feature/your-feature-name
   ```

6. **Create a pull request**: Go to the [original repository](https://github.com/elispeigel/spocker), and click on the "New pull request" button. Choose your fork and the branch you've worked on, then create the pull request. Describe your changes and reference the issue you were working on.

## Code Style and Best Practices

Please follow the coding style and best practices used in the project. This includes proper formatting, commenting, and adhering to the principles of clean code. This helps maintain the readability and maintainability of the project.

## Reporting Bugs and Requesting Features

If you encounter a bug or have a suggestion for a new feature, please create a new issue in the [issues tab](https://github.com/elispeigel/spocker/issues). Provide a clear and concise description of the issue or feature, along with any relevant information.

## Questions and Support

If you have any questions or need assistance, you can reach out to the project maintainers by creating a new issue or contacting them directly.

---

Thank you for contributing to Spocker! Your efforts help make this project better for everyone.