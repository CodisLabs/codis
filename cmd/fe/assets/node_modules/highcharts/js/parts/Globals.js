// encapsulated variables
var UNDEFINED,
	win = w || window,
	doc = win.document,
	math = Math,
	mathRound = math.round,
	mathFloor = math.floor,
	mathCeil = math.ceil,
	mathMax = math.max,
	mathMin = math.min,
	mathAbs = math.abs,
	mathCos = math.cos,
	mathSin = math.sin,
	mathPI = math.PI,
	deg2rad = mathPI * 2 / 360,


	// some variables
	userAgent = navigator.userAgent,
	isOpera = win.opera,
	isMS = /(msie|trident|edge)/i.test(userAgent) && !isOpera,
	docMode8 = doc.documentMode === 8,
	isWebKit = !isMS && /AppleWebKit/.test(userAgent),
	isFirefox = /Firefox/.test(userAgent),
	isTouchDevice = /(Mobile|Android|Windows Phone)/.test(userAgent),
	SVG_NS = 'http://www.w3.org/2000/svg',
	hasSVG = !!doc.createElementNS && !!doc.createElementNS(SVG_NS, 'svg').createSVGRect,
	hasBidiBug = isFirefox && parseInt(userAgent.split('Firefox/')[1], 10) < 4, // issue #38
	useCanVG = !hasSVG && !isMS && !!doc.createElement('canvas').getContext,
	Renderer,
	hasTouch,
	symbolSizes = {},
	idCounter = 0,
	garbageBin,
	defaultOptions,
	dateFormat, // function
	pathAnim,
	timeUnits,
	noop = function () {},
	charts = [],
	chartCount = 0,
	PRODUCT = '@product.name@',
	VERSION = '@product.version@',

	// some constants for frequently used strings
	DIV = 'div',
	ABSOLUTE = 'absolute',
	RELATIVE = 'relative',
	HIDDEN = 'hidden',
	PREFIX = 'highcharts-',
	VISIBLE = 'visible',
	PX = 'px',
	NONE = 'none',
	M = 'M',
	L = 'L',
	numRegex = /^[0-9]+$/,
	NORMAL_STATE = '',
	HOVER_STATE = 'hover',
	SELECT_STATE = 'select',
	marginNames = ['plotTop', 'marginRight', 'marginBottom', 'plotLeft'],

	// Object for extending Axis
	AxisPlotLineOrBandExtension,

	// constants for attributes
	STROKE_WIDTH = 'stroke-width',

	// time methods, changed based on whether or not UTC is used
	Date,  // Allow using a different Date class
	makeTime,
	timezoneOffset,
	getTimezoneOffset,
	getMinutes,
	getHours,
	getDay,
	getDate,
	getMonth,
	getFullYear,
	setMilliseconds,
	setSeconds,
	setMinutes,
	setHours,
	setDate,
	setMonth,
	setFullYear,


	// lookup over the types and the associated classes
	seriesTypes = {},
	Highcharts;

/**
 * Provide error messages for debugging, with links to online explanation
 */
function error(code, stop) {
	var msg = 'Highcharts error #' + code + ': www.highcharts.com/errors/' + code;
	if (stop) {
		throw new Error(msg);
	}
	// else ...
	if (win.console) {
		console.log(msg); // eslint-disable-line no-console
	}
}

// The Highcharts namespace
Highcharts = win.Highcharts ? error(16, true) : function (adapter) {
	Highcharts.loadAdapter(adapter);
	return Highcharts;
};

Highcharts.seriesTypes = seriesTypes;
