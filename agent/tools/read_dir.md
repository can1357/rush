Shows files and subdirectories in tree structure for exploring project organization.

<usage>
- Provide path to list (defaults to current working directory)
- Optional glob patterns to ignore
- Results displayed in tree structure
</usage>

<features>
- Hierarchical view of files and directories
- Auto-skips hidden files/directories (starting with '.')
- Skips common system directories like __pycache__
- Can filter files matching specific patterns
</features>

<limitations>
- Results limited to 1000 files
- Large directories truncated
- No file sizes or permissions shown
- Cannot recursively list all directories in large projects
</limitations>

<tips>
- Use Glob for finding files by name patterns instead of browsing
- Use Grep for searching file contents
- Combine with other tools for effective exploration
</tips>
