#!/bin/bash

# Function to create a random file with random content
generate_random_file() {
    local dir=$1
    local min_size=$2
    local max_size=$3

    # Generate a random file name
    local file_name=$(tr -dc 'a-zA-Z0-9' </dev/urandom | head -c 8)
    
    # Determine a random size for the file, between 1 + min_size and max_size bytes
    local file_size=$((RANDOM % max_size + min_size + 1))
    
    # Create the file with random content
    head -c "$file_size" /dev/urandom > "${dir}/${file_name}.txt"
}

# Function to create a random directory nesting
create_random_nesting() {
    local base_dir=$1
    local depth=$((RANDOM % 5 + 1))  # Random depth between 1 and 5

    local current_dir=$base_dir
    for (( i = 0; i < depth; i++ )); do
        if [ $((RANDOM % 2)) -eq 0 ] && [ -d "$current_dir" ]; then
            # Randomly reuse an existing directory
            current_dir=$(find "$base_dir" -type d | shuf -n 1)
        else
            # Create a new directory
            local dir_name=$(tr -dc 'a-zA-Z0-9' </dev/urandom | head -c 8)
            current_dir="${current_dir}/${dir_name}"
            mkdir -p "$current_dir"
        fi
    done

    echo "$current_dir"
}

# Main script execution
if [ $# -ne 4 ]; then
    echo "Usage: $0 <target_directory> <number_of_files> <min_file_size> <max_file_size>"
    exit 1
fi

target_directory=$1
total_files=$2
min_size=$3
max_size=$4

# Check if the target directory exists, create it if not
if [ ! -d "$target_directory" ]; then
    mkdir -p "$target_directory"
fi

# Create files with random nesting and directory reuse
for (( i = 0; i < total_files; i++ )); do
    random_dir=$(create_random_nesting "$target_directory")
    generate_random_file "$random_dir" "$min_size" "$max_size"
done

echo "Generated $total_files files with random nesting and reused directories in $target_directory."
