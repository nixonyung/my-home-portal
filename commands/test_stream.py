import sys
import time


def main():
    for i in range(1, 7):
        print(f"{i}: {sys.argv[1:]}", flush=True)
        time.sleep(0.25)


if __name__ == "__main__":
    main()
