find . -name "*.go" -not -path "./node_modules/*" -print | xargs sed -i 's|"github.com/mgutz/dat"|"github.com/mgutz/dat/dat"|g'


