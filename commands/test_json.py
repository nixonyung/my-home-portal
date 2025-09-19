import json
import sys


def main():
    print(
        json.dumps(
            sys.argv[1:],
            indent=2,
            ensure_ascii=False,
        )
    )


if __name__ == "__main__":
    main()
