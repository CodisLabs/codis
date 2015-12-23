
// global variables
extend(Highcharts, {

	// Constructors
	Color: Color,
	Point: Point,
	Tick: Tick,
	Renderer: Renderer,
	SVGElement: SVGElement,
	SVGRenderer: SVGRenderer,

	// Various
	arrayMin: arrayMin,
	arrayMax: arrayMax,
	charts: charts,
	dateFormat: dateFormat,
	error: error,
	format: format,
	pathAnim: pathAnim,
	getOptions: getOptions,
	hasBidiBug: hasBidiBug,
	isTouchDevice: isTouchDevice,
	setOptions: setOptions,
	addEvent: addEvent,
	removeEvent: removeEvent,
	createElement: createElement,
	discardElement: discardElement,
	css: css,
	each: each,
	map: map,
	merge: merge,
	splat: splat,
	stableSort: stableSort,
	extendClass: extendClass,
	pInt: pInt,
	svg: hasSVG,
	canvas: useCanVG,
	vml: !hasSVG && !useCanVG,
	product: PRODUCT,
	version: VERSION
});
