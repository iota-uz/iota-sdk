git checkout build
templ generate
git merge main
git add -f '*_templ.go'
git commit -m"add build files"
git push
git checkout main