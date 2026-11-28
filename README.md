# go-git-aico

- Inspired by [git-aico](https://github.com/hirokidaichi/git-aico)
- The majority of the code was written using [aider](https://github.com/paul-gauthier/aider) and [cluade code](https://docs.anthropic.com/en/docs/claude-code/overview)

#### Installation

To install the tool using `go install`, run the following command:

```sh
go install github.com/komapotter/go-git-aico/cmd/git-aico@latest
```

This will install the `git-aico` executable in your `$GOPATH/bin` directory.

1. Ensure you have staged changes in your git repository by running `git add`.
2. Run the tool using `git aico` to generate commit message suggestions.
3. If you want verbose output, which includes the raw response from the AI model, run the tool with the `-v` flag like this: `git aico -v`.
4. To print the installed version and exit, run `git aico -V`.
5. The tool will present you with a list of commit message suggestions based on the staged changes.
6. Select the appropriate commit message by entering the number corresponding to the suggestion.
7. The tool will automatically commit your staged changes with the selected commit message.

### Authentication

API keys can live in environment variables (CI and one-off overrides) or in the OS secure store:

- macOS: Keychain
- Linux: Secret Service (when available)
- Windows: Credential Manager (Wincred)

On macOS, the first Keychain access may show a permission dialog.

```sh
git-aico auth register              # paste the OpenAI API key (no echo)
git-aico auth status                # shows whether a key is registered (never prints the key)
git-aico auth remove
```

`login` / `logout` are not used; the commands are **register** / **remove** / **status**.

API keys are never written to a config file. Existing OpenAI keys stored in the OS keyring remain readable.

### Credential resolution order

When generating a commit message, credentials are resolved in this order:

1. Environment variable `OPENAI_API_KEY` wins if set
2. Otherwise the OS keyring
3. Otherwise an error suggesting `git-aico auth register`

Existing env-only workflows keep working without running `auth register`.

### Environment Variables

These remain optional when the matching key is registered in the OS keyring.

#### General Configuration
- `NUM_CANDIDATES`: The number of commit message candidates to generate (default: 3)

#### OpenAI Configuration
- `OPENAI_API_KEY`: Your OpenAI API key (required unless registered via `git-aico auth register`)
- `OPENAI_MODEL`: The OpenAI model to use (default: gpt-6-luna)
- `OPENAI_TEMPERATURE`: The OpenAI temperature parameter (default: 0.1). Sent for non-reasoning models, or when `OPENAI_REASONING_EFFORT=none`; omitted otherwise
- `OPENAI_MAX_TOKENS`: The maximum number of completion tokens for OpenAI, mapped to `max_completion_tokens` (default: 450)
- `OPENAI_REASONING_EFFORT`: Reasoning effort for reasoning models (gpt-6 / gpt-5 / o-series). Default: none. Allowed: none, low, medium, high, xhigh, max. Omitted from the request for non-reasoning models such as gpt-4o

Example of setting environment variables:

```sh
export OPENAI_API_KEY="your_openai_api_key"
export NUM_CANDIDATES=3
export OPENAI_MODEL="gpt-6-luna"
export OPENAI_TEMPERATURE=0.1
export OPENAI_MAX_TOKENS=450
export OPENAI_REASONING_EFFORT=none
```
