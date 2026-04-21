import os
import re
import csv
import argparse
from sync_analyser import analyze_sync

def find_pairs(directory):
    """Finds pairs of left_*.mkv and right_*.mkv files in the given directory."""
    # Regex to match left_YYYYMMDD_HHMMSS
    pattern = re.compile(r"left_(\d{8}_\d{6})\.mkv")
    files = os.listdir(directory)
    mkv_files = [(f, match.group(1)) for f in files if (match := pattern.match(f))]
    
    valid_pairs = []
    for file, ts in mkv_files:
        if f"right_{ts}.mkv" in files:
            valid_pairs.append({
                "timestamp": ts,
                "left": os.path.join(directory, file),
                "right": os.path.join(directory, f"right_{ts}.mkv")
            })
            
    return valid_pairs

def main():
    parser = argparse.ArgumentParser(description="Bulk analyze stereo video synchronization.")
    parser.add_argument("directory", help="Directory containing the .mkv video files.")
    parser.add_argument("--output", default="sync_results.csv", help="Output CSV filename (default: sync_results.csv).")
    args = parser.parse_args()
    
    if not os.path.isdir(args.directory):
        print(f"Error: {args.directory} is not a directory.")
        return

    pairs = find_pairs(args.directory)
    if not pairs:
        print("No valid left/right video pairs found.")
        return

    print(f"Found {len(pairs)} pairs to analyze.")
    
    results_list = []
    
    for i, pair in enumerate(pairs):
        print(f"[{i+1}/{len(pairs)}] Analyzing {pair['timestamp']}...")
        try:
            res = analyze_sync(pair["left"], pair["right"], headless=True)
            if res:
                results_list.append({
                    "Timestamp": pair["timestamp"],
                    "Left File": os.path.basename(pair["left"]),
                    "Right File": os.path.basename(pair["right"]),
                    "Total Frames": res["count"],
                    "Filtered Frames": res["filtered_count"],
                    "Mean Offset (ms)": f"{res['mean']:+.2f}",
                    "Jitter (ms)": f"{res['std']:.2f}"
                })
        except Exception as e:
            print(f"Error analyzing {pair['timestamp']}: {e}")

    if results_list:
        keys = results_list[0].keys()
        with open(args.output, "w", newline="") as f:
            dict_writer = csv.DictWriter(f, fieldnames=keys)
            dict_writer.writeheader()
            dict_writer.writerows(results_list)
        print(f"\nResults saved to {args.output}")
    else:
        print("\nNo results to save.")

if __name__ == "__main__":
    main()
