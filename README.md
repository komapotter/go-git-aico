# go-git-aico

- Inspired by [git-aico](https://github.com/hirokidaichi/git-aico)
- The majority of the code was written using [aider](https://github.com/paul-gauthier/aider)

#### Installation

To install the tool using `go install`, run the following command:

```sh
go install github.com/komapotter/go-git-aico/cmd/git-aico@latest
```

This will install the `git-aico` executable in your `$GOPATH/bin` directory.

1. Ensure you have staged changes in your git repository by running `git add`.
2. Run the tool using `git aico` to generate commit message suggestions.
3. If you want verbose output, which includes the raw response from OpenAI, run the tool with the `-v` flag like this: `git aico -v`.
4. The tool will present you with a list of commit message suggestions based on the staged changes.
5. Select the appropriate commit message by entering the number corresponding to the suggestion.
6. The tool will automatically commit your staged changes with the selected commit message.

### Environment Variables

To use this tool, you need to set the following environment variables:

- `OPENAI_API_KEY`: Your OpenAI API key
- `NUM_CANDIDATES`: The number of commit message candidates to generate (default: 3)
- `OPENAI_MODEL`: The OpenAI model to use (default: gpt-4o)
- `OPENAI_TEMPERATURE`: The OpenAI temperature parameter (default: 0.1)
- `OPENAI_MAX_TOKENS`: The maximum number of tokens for OpenAI (default: 450)

Example of setting environment variables:

```sh
export OPENAI_API_KEY="your_openai_api_key"
export NUM_CANDIDATES=3
export OPENAI_MODEL="gpt-4o"
export OPENAI_TEMPERATURE=0.1
export OPENAI_MAX_TOKENS=450
```
