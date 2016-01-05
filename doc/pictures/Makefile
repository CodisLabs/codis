all:
	@mkdir -p tmp
	@xelatex -output-directory tmp architecture.tex
	@convert -density 300 tmp/architecture.pdf -quality 90 -background white -alpha off architecture.png

