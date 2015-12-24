/**
 * The chart class
 * @param {Object} options
 * @param {Function} callback Function to run when the chart has loaded
 */
var Chart = Highcharts.Chart = function () {
	this.init.apply(this, arguments);
};

Chart.prototype = {

	/**
	 * Hook for modules
	 */
	callbacks: [],

	/**
	 * Initialize the chart
	 */
	init: function (userOptions, callback) {

		// Handle regular options
		var options,
			seriesOptions = userOptions.series; // skip merging data points to increase performance

		userOptions.series = null;
		options = merge(defaultOptions, userOptions); // do the merge
		options.series = userOptions.series = seriesOptions; // set back the series data
		this.userOptions = userOptions;

		var optionsChart = options.chart;

		// Create margin & spacing array
		this.margin = this.splashArray('margin', optionsChart);
		this.spacing = this.splashArray('spacing', optionsChart);

		var chartEvents = optionsChart.events;

		//this.runChartClick = chartEvents && !!chartEvents.click;
		this.bounds = { h: {}, v: {} }; // Pixel data bounds for touch zoom

		this.callback = callback;
		this.isResizing = 0;
		this.options = options;
		//chartTitleOptions = UNDEFINED;
		//chartSubtitleOptions = UNDEFINED;

		this.axes = [];
		this.series = [];
		this.hasCartesianSeries = optionsChart.showAxes;
		//this.axisOffset = UNDEFINED;
		//this.maxTicks = UNDEFINED; // handle the greatest amount of ticks on grouped axes
		//this.inverted = UNDEFINED;
		//this.loadingShown = UNDEFINED;
		//this.container = UNDEFINED;
		//this.chartWidth = UNDEFINED;
		//this.chartHeight = UNDEFINED;
		//this.marginRight = UNDEFINED;
		//this.marginBottom = UNDEFINED;
		//this.containerWidth = UNDEFINED;
		//this.containerHeight = UNDEFINED;
		//this.oldChartWidth = UNDEFINED;
		//this.oldChartHeight = UNDEFINED;

		//this.renderTo = UNDEFINED;
		//this.renderToClone = UNDEFINED;

		//this.spacingBox = UNDEFINED

		//this.legend = UNDEFINED;

		// Elements
		//this.chartBackground = UNDEFINED;
		//this.plotBackground = UNDEFINED;
		//this.plotBGImage = UNDEFINED;
		//this.plotBorder = UNDEFINED;
		//this.loadingDiv = UNDEFINED;
		//this.loadingSpan = UNDEFINED;

		var chart = this,
			eventType;

		// Add the chart to the global lookup
		chart.index = charts.length;
		charts.push(chart);
		chartCount++;

		// Set up auto resize
		if (optionsChart.reflow !== false) {
			addEvent(chart, 'load', function () {
				chart.initReflow();
			});
		}

		// Chart event handlers
		if (chartEvents) {
			for (eventType in chartEvents) {
				addEvent(chart, eventType, chartEvents[eventType]);
			}
		}

		chart.xAxis = [];
		chart.yAxis = [];

		// Expose methods and variables
		chart.animation = useCanVG ? false : pick(optionsChart.animation, true);
		chart.pointCount = chart.colorCounter = chart.symbolCounter = 0;

		chart.firstRender();
	},

	/**
	 * Initialize an individual series, called internally before render time
	 */
	initSeries: function (options) {
		var chart = this,
			optionsChart = chart.options.chart,
			type = options.type || optionsChart.type || optionsChart.defaultSeriesType,
			series,
			constr = seriesTypes[type];

		// No such series type
		if (!constr) {
			error(17, true);
		}

		series = new constr();
		series.init(this, options);
		return series;
	},

	/**
	 * Check whether a given point is within the plot area
	 *
	 * @param {Number} plotX Pixel x relative to the plot area
	 * @param {Number} plotY Pixel y relative to the plot area
	 * @param {Boolean} inverted Whether the chart is inverted
	 */
	isInsidePlot: function (plotX, plotY, inverted) {
		var x = inverted ? plotY : plotX,
			y = inverted ? plotX : plotY;

		return x >= 0 &&
			x <= this.plotWidth &&
			y >= 0 &&
			y <= this.plotHeight;
	},

	/**
	 * Redraw legend, axes or series based on updated data
	 *
	 * @param {Boolean|Object} animation Whether to apply animation, and optionally animation
	 *    configuration
	 */
	redraw: function (animation) {
		var chart = this,
			axes = chart.axes,
			series = chart.series,
			pointer = chart.pointer,
			legend = chart.legend,
			redrawLegend = chart.isDirtyLegend,
			hasStackedSeries,
			hasDirtyStacks,
			hasCartesianSeries = chart.hasCartesianSeries,
			isDirtyBox = chart.isDirtyBox,
			seriesLength = series.length,
			i = seriesLength,
			serie,
			renderer = chart.renderer,
			isHiddenChart = renderer.isHidden(),
			afterRedraw = [];

		setAnimation(animation, chart);

		if (isHiddenChart) {
			chart.cloneRenderTo();
		}

		// Adjust title layout (reflow multiline text)
		chart.layOutTitles();

		// link stacked series
		while (i--) {
			serie = series[i];

			if (serie.options.stacking) {
				hasStackedSeries = true;

				if (serie.isDirty) {
					hasDirtyStacks = true;
					break;
				}
			}
		}
		if (hasDirtyStacks) { // mark others as dirty
			i = seriesLength;
			while (i--) {
				serie = series[i];
				if (serie.options.stacking) {
					serie.isDirty = true;
				}
			}
		}

		// Handle updated data in the series
		each(series, function (serie) {
			if (serie.isDirty) {
				if (serie.options.legendType === 'point') {
					if (serie.updateTotals) {
						serie.updateTotals();
					}
					redrawLegend = true;
				}
			}
		});

		// handle added or removed series
		if (redrawLegend && legend.options.enabled) { // series or pie points are added or removed
			// draw legend graphics
			legend.render();

			chart.isDirtyLegend = false;
		}

		// reset stacks
		if (hasStackedSeries) {
			chart.getStacks();
		}


		if (hasCartesianSeries) {
			if (!chart.isResizing) {

				// reset maxTicks
				chart.maxTicks = null;

				// set axes scales
				each(axes, function (axis) {
					axis.setScale();
				});
			}
		}

		chart.getMargins(); // #3098

		if (hasCartesianSeries) {
			// If one axis is dirty, all axes must be redrawn (#792, #2169)
			each(axes, function (axis) {
				if (axis.isDirty) {
					isDirtyBox = true;
				}
			});

			// redraw axes
			each(axes, function (axis) {

				// Fire 'afterSetExtremes' only if extremes are set
				var key = axis.min + ',' + axis.max;
				if (axis.extKey !== key) { // #821, #4452
					axis.extKey = key;
					afterRedraw.push(function () { // prevent a recursive call to chart.redraw() (#1119)
						fireEvent(axis, 'afterSetExtremes', extend(axis.eventArgs, axis.getExtremes())); // #747, #751
						delete axis.eventArgs;
					});
				}
				if (isDirtyBox || hasStackedSeries) {
					axis.redraw();
				}
			});
		}

		// the plot areas size has changed
		if (isDirtyBox) {
			chart.drawChartBox();
		}


		// redraw affected series
		each(series, function (serie) {
			if (serie.isDirty && serie.visible &&
					(!serie.isCartesian || serie.xAxis)) { // issue #153
				serie.redraw();
			}
		});

		// move tooltip or reset
		if (pointer) {
			pointer.reset(true);
		}

		// redraw if canvas
		renderer.draw();

		// fire the event
		fireEvent(chart, 'redraw'); // jQuery breaks this when calling it from addEvent. Overwrites chart.redraw

		if (isHiddenChart) {
			chart.cloneRenderTo(true);
		}

		// Fire callbacks that are put on hold until after the redraw
		each(afterRedraw, function (callback) {
			callback.call();
		});
	},

	/**
	 * Get an axis, series or point object by id.
	 * @param id {String} The id as given in the configuration options
	 */
	get: function (id) {
		var chart = this,
			axes = chart.axes,
			series = chart.series;

		var i,
			j,
			points;

		// search axes
		for (i = 0; i < axes.length; i++) {
			if (axes[i].options.id === id) {
				return axes[i];
			}
		}

		// search series
		for (i = 0; i < series.length; i++) {
			if (series[i].options.id === id) {
				return series[i];
			}
		}

		// search points
		for (i = 0; i < series.length; i++) {
			points = series[i].points || [];
			for (j = 0; j < points.length; j++) {
				if (points[j].id === id) {
					return points[j];
				}
			}
		}
		return null;
	},

	/**
	 * Create the Axis instances based on the config options
	 */
	getAxes: function () {
		var chart = this,
			options = this.options,
			xAxisOptions = options.xAxis = splat(options.xAxis || {}),
			yAxisOptions = options.yAxis = splat(options.yAxis || {}),
			optionsArray;

		// make sure the options are arrays and add some members
		each(xAxisOptions, function (axis, i) {
			axis.index = i;
			axis.isX = true;
		});

		each(yAxisOptions, function (axis, i) {
			axis.index = i;
		});

		// concatenate all axis options into one array
		optionsArray = xAxisOptions.concat(yAxisOptions);

		each(optionsArray, function (axisOptions) {
			new Axis(chart, axisOptions); // eslint-disable-line no-new
		});
	},


	/**
	 * Get the currently selected points from all series
	 */
	getSelectedPoints: function () {
		var points = [];
		each(this.series, function (serie) {
			points = points.concat(grep(serie.points || [], function (point) {
				return point.selected;
			}));
		});
		return points;
	},

	/**
	 * Get the currently selected series
	 */
	getSelectedSeries: function () {
		return grep(this.series, function (serie) {
			return serie.selected;
		});
	},

	/**
	 * Show the title and subtitle of the chart
	 *
	 * @param titleOptions {Object} New title options
	 * @param subtitleOptions {Object} New subtitle options
	 *
	 */
	setTitle: function (titleOptions, subtitleOptions, redraw) {
		var chart = this,
			options = chart.options,
			chartTitleOptions,
			chartSubtitleOptions;

		chartTitleOptions = options.title = merge(options.title, titleOptions);
		chartSubtitleOptions = options.subtitle = merge(options.subtitle, subtitleOptions);

		// add title and subtitle
		each([
			['title', titleOptions, chartTitleOptions],
			['subtitle', subtitleOptions, chartSubtitleOptions]
		], function (arr) {
			var name = arr[0],
				title = chart[name],
				titleOptions = arr[1],
				chartTitleOptions = arr[2];

			if (title && titleOptions) {
				chart[name] = title = title.destroy(); // remove old
			}

			if (chartTitleOptions && chartTitleOptions.text && !title) {
				chart[name] = chart.renderer.text(
					chartTitleOptions.text,
					0,
					0,
					chartTitleOptions.useHTML
				)
				.attr({
					align: chartTitleOptions.align,
					'class': PREFIX + name,
					zIndex: chartTitleOptions.zIndex || 4
				})
				.css(chartTitleOptions.style)
				.add();
			}
		});
		chart.layOutTitles(redraw);
	},

	/**
	 * Lay out the chart titles and cache the full offset height for use in getMargins
	 */
	layOutTitles: function (redraw) {
		var titleOffset = 0,
			title = this.title,
			subtitle = this.subtitle,
			options = this.options,
			titleOptions = options.title,
			subtitleOptions = options.subtitle,
			requiresDirtyBox,
			renderer = this.renderer,
			autoWidth = this.spacingBox.width - 44; // 44 makes room for default context button

		if (title) {
			title
				.css({ width: (titleOptions.width || autoWidth) + PX })
				.align(extend({
					y: renderer.fontMetrics(titleOptions.style.fontSize, title).b - 3
				}, titleOptions), false, 'spacingBox');

			if (!titleOptions.floating && !titleOptions.verticalAlign) {
				titleOffset = title.getBBox().height;
			}
		}
		if (subtitle) {
			subtitle
				.css({ width: (subtitleOptions.width || autoWidth) + PX })
				.align(extend({
					y: titleOffset + (titleOptions.margin - 13) + renderer.fontMetrics(subtitleOptions.style.fontSize, title).b
				}, subtitleOptions), false, 'spacingBox');

			if (!subtitleOptions.floating && !subtitleOptions.verticalAlign) {
				titleOffset = mathCeil(titleOffset + subtitle.getBBox().height);
			}
		}

		requiresDirtyBox = this.titleOffset !== titleOffset;
		this.titleOffset = titleOffset; // used in getMargins

		if (!this.isDirtyBox && requiresDirtyBox) {
			this.isDirtyBox = requiresDirtyBox;
			// Redraw if necessary (#2719, #2744)
			if (this.hasRendered && pick(redraw, true) && this.isDirtyBox) {
				this.redraw();
			}
		}
	},

	/**
	 * Get chart width and height according to options and container size
	 */
	getChartSize: function () {
		var chart = this,
			optionsChart = chart.options.chart,
			widthOption = optionsChart.width,
			heightOption = optionsChart.height,
			renderTo = chart.renderToClone || chart.renderTo;

		// get inner width and height from jQuery (#824)
		if (!defined(widthOption)) {
			chart.containerWidth = adapterRun(renderTo, 'width');
		}
		if (!defined(heightOption)) {
			chart.containerHeight = adapterRun(renderTo, 'height');
		}

		chart.chartWidth = mathMax(0, widthOption || chart.containerWidth || 600); // #1393, 1460
		chart.chartHeight = mathMax(0, pick(heightOption,
			// the offsetHeight of an empty container is 0 in standard browsers, but 19 in IE7:
			chart.containerHeight > 19 ? chart.containerHeight : 400));
	},

	/**
	 * Create a clone of the chart's renderTo div and place it outside the viewport to allow
	 * size computation on chart.render and chart.redraw
	 */
	cloneRenderTo: function (revert) {
		var clone = this.renderToClone,
			container = this.container;

		// Destroy the clone and bring the container back to the real renderTo div
		if (revert) {
			if (clone) {
				this.renderTo.appendChild(container);
				discardElement(clone);
				delete this.renderToClone;
			}

		// Set up the clone
		} else {
			if (container && container.parentNode === this.renderTo) {
				this.renderTo.removeChild(container); // do not clone this
			}
			this.renderToClone = clone = this.renderTo.cloneNode(0);
			css(clone, {
				position: ABSOLUTE,
				top: '-9999px',
				display: 'block' // #833
			});
			if (clone.style.setProperty) { // #2631
				clone.style.setProperty('display', 'block', 'important');
			}
			doc.body.appendChild(clone);
			if (container) {
				clone.appendChild(container);
			}
		}
	},

	/**
	 * Get the containing element, determine the size and create the inner container
	 * div to hold the chart
	 */
	getContainer: function () {
		var chart = this,
			container,
			options = chart.options,
			optionsChart = options.chart,
			chartWidth,
			chartHeight,
			renderTo,
			indexAttrName = 'data-highcharts-chart',
			oldChartIndex,
			Ren,
			containerId;

		chart.renderTo = renderTo = optionsChart.renderTo;
		containerId = PREFIX + idCounter++;

		if (isString(renderTo)) {
			chart.renderTo = renderTo = doc.getElementById(renderTo);
		}

		// Display an error if the renderTo is wrong
		if (!renderTo) {
			error(13, true);
		}

		// If the container already holds a chart, destroy it. The check for hasRendered is there
		// because web pages that are saved to disk from the browser, will preserve the data-highcharts-chart
		// attribute and the SVG contents, but not an interactive chart. So in this case,
		// charts[oldChartIndex] will point to the wrong chart if any (#2609).
		oldChartIndex = pInt(attr(renderTo, indexAttrName));
		if (!isNaN(oldChartIndex) && charts[oldChartIndex] && charts[oldChartIndex].hasRendered) {
			charts[oldChartIndex].destroy();
		}

		// Make a reference to the chart from the div
		attr(renderTo, indexAttrName, chart.index);

		// remove previous chart
		renderTo.innerHTML = '';

		// If the container doesn't have an offsetWidth, it has or is a child of a node
		// that has display:none. We need to temporarily move it out to a visible
		// state to determine the size, else the legend and tooltips won't render
		// properly. The allowClone option is used in sparklines as a micro optimization,
		// saving about 1-2 ms each chart.
		if (!optionsChart.skipClone && !renderTo.offsetWidth) {
			chart.cloneRenderTo();
		}

		// get the width and height
		chart.getChartSize();
		chartWidth = chart.chartWidth;
		chartHeight = chart.chartHeight;

		// create the inner container
		chart.container = container = createElement(DIV, {
				className: PREFIX + 'container' +
					(optionsChart.className ? ' ' + optionsChart.className : ''),
				id: containerId
			}, extend({
				position: RELATIVE,
				overflow: HIDDEN, // needed for context menu (avoid scrollbars) and
					// content overflow in IE
				width: chartWidth + PX,
				height: chartHeight + PX,
				textAlign: 'left',
				lineHeight: 'normal', // #427
				zIndex: 0, // #1072
				'-webkit-tap-highlight-color': 'rgba(0,0,0,0)'
			}, optionsChart.style),
			chart.renderToClone || renderTo
		);

		// cache the cursor (#1650)
		chart._cursor = container.style.cursor;

		// Initialize the renderer
		Ren = Highcharts[optionsChart.renderer] || Renderer;
		chart.renderer = new Ren(
			container,
			chartWidth,
			chartHeight,
			optionsChart.style,
			optionsChart.forExport,
			options.exporting && options.exporting.allowHTML
		);

		if (useCanVG) {
			// If we need canvg library, extend and configure the renderer
			// to get the tracker for translating mouse events
			chart.renderer.create(chart, container, chartWidth, chartHeight);
		}
		// Add a reference to the charts index
		chart.renderer.chartIndex = chart.index;
	},

	/**
	 * Calculate margins by rendering axis labels in a preliminary position. Title,
	 * subtitle and legend have already been rendered at this stage, but will be
	 * moved into their final positions
	 */
	getMargins: function (skipAxes) {
		var chart = this,
			spacing = chart.spacing,
			margin = chart.margin,
			titleOffset = chart.titleOffset;

		chart.resetMargins();

		// Adjust for title and subtitle
		if (titleOffset && !defined(margin[0])) {
			chart.plotTop = mathMax(chart.plotTop, titleOffset + chart.options.title.margin + spacing[0]);
		}

		// Adjust for legend
		chart.legend.adjustMargins(margin, spacing);

		// adjust for scroller
		if (chart.extraBottomMargin) {
			chart.marginBottom += chart.extraBottomMargin;
		}
		if (chart.extraTopMargin) {
			chart.plotTop += chart.extraTopMargin;
		}
		if (!skipAxes) {
			this.getAxisMargins();
		}
	},

	getAxisMargins: function () {

		var chart = this,
			axisOffset = chart.axisOffset = [0, 0, 0, 0], // top, right, bottom, left
			margin = chart.margin;

		// pre-render axes to get labels offset width
		if (chart.hasCartesianSeries) {
			each(chart.axes, function (axis) {
				if (axis.visible) {
					axis.getOffset();
				}
			});
		}

		// Add the axis offsets
		each(marginNames, function (m, side) {
			if (!defined(margin[side])) {
				chart[m] += axisOffset[side];
			}
		});

		chart.setChartSize();

	},

	/**
	 * Resize the chart to its container if size is not explicitly set
	 */
	reflow: function (e) {
		var chart = this,
			optionsChart = chart.options.chart,
			renderTo = chart.renderTo,
			width = optionsChart.width || adapterRun(renderTo, 'width'),
			height = optionsChart.height || adapterRun(renderTo, 'height'),
			target = e ? e.target : win; // #805 - MooTools doesn't supply e

		// Width and height checks for display:none. Target is doc in IE8 and Opera,
		// win in Firefox, Chrome and IE9.
		if (!chart.hasUserSize && !chart.isPrinting && width && height && (target === win || target === doc)) { // #1093
			if (width !== chart.containerWidth || height !== chart.containerHeight) {
				clearTimeout(chart.reflowTimeout);
				// When called from window.resize, e is set, else it's called directly (#2224)
				chart.reflowTimeout = syncTimeout(function () {
					if (chart.container) { // It may have been destroyed in the meantime (#1257)
						chart.setSize(width, height, false);
						chart.hasUserSize = null;
					}
				}, e ? 100 : 0);
			}
			chart.containerWidth = width;
			chart.containerHeight = height;
		}
	},

	/**
	 * Add the event handlers necessary for auto resizing
	 */
	initReflow: function () {
		var chart = this,
			reflow = function (e) {
				chart.reflow(e);
			};


		addEvent(win, 'resize', reflow);
		addEvent(chart, 'destroy', function () {
			removeEvent(win, 'resize', reflow);
		});
	},

	/**
	 * Resize the chart to a given width and height
	 * @param {Number} width
	 * @param {Number} height
	 * @param {Object|Boolean} animation
	 */
	setSize: function (width, height, animation) {
		var chart = this,
			chartWidth,
			chartHeight,
			renderer = chart.renderer,
			globalAnimation;

		// Handle the isResizing counter
		chart.isResizing += 1;
		
		// set the animation for the current process
		setAnimation(animation, chart);

		chart.oldChartHeight = chart.chartHeight;
		chart.oldChartWidth = chart.chartWidth;
		if (defined(width)) {
			chart.chartWidth = chartWidth = mathMax(0, mathRound(width));
			chart.hasUserSize = !!chartWidth;
		}
		if (defined(height)) {
			chart.chartHeight = chartHeight = mathMax(0, mathRound(height));
		}

		// Resize the container with the global animation applied if enabled (#2503)
		globalAnimation = renderer.globalAnimation;
		(globalAnimation ? animate : css)(chart.container, {
			width: chartWidth + PX,
			height: chartHeight + PX
		}, globalAnimation);

		chart.setChartSize(true);
		renderer.setSize(chartWidth, chartHeight, animation);

		// handle axes
		chart.maxTicks = null;
		each(chart.axes, function (axis) {
			axis.isDirty = true;
			axis.setScale();
		});

		// make sure non-cartesian series are also handled
		each(chart.series, function (serie) {
			serie.isDirty = true;
		});

		chart.isDirtyLegend = true; // force legend redraw
		chart.isDirtyBox = true; // force redraw of plot and chart border

		chart.layOutTitles(); // #2857
		chart.getMargins();

		chart.redraw(animation);


		chart.oldChartHeight = null;
		fireEvent(chart, 'resize');

		// Fire endResize and set isResizing back. If animation is disabled, fire without delay
		globalAnimation = renderer.globalAnimation; // Reassign it before using it, it may have changed since the top of this function.
		syncTimeout(function () {
			if (chart) {
				fireEvent(chart, 'endResize', null, function () {
					chart.isResizing -= 1;
				});
			}
		}, globalAnimation === false ? 0 : ((globalAnimation && globalAnimation.duration) || 500));
	},

	/**
	 * Set the public chart properties. This is done before and after the pre-render
	 * to determine margin sizes
	 */
	setChartSize: function (skipAxes) {
		var chart = this,
			inverted = chart.inverted,
			renderer = chart.renderer,
			chartWidth = chart.chartWidth,
			chartHeight = chart.chartHeight,
			optionsChart = chart.options.chart,
			spacing = chart.spacing,
			clipOffset = chart.clipOffset,
			clipX,
			clipY,
			plotLeft,
			plotTop,
			plotWidth,
			plotHeight,
			plotBorderWidth;

		chart.plotLeft = plotLeft = mathRound(chart.plotLeft);
		chart.plotTop = plotTop = mathRound(chart.plotTop);
		chart.plotWidth = plotWidth = mathMax(0, mathRound(chartWidth - plotLeft - chart.marginRight));
		chart.plotHeight = plotHeight = mathMax(0, mathRound(chartHeight - plotTop - chart.marginBottom));

		chart.plotSizeX = inverted ? plotHeight : plotWidth;
		chart.plotSizeY = inverted ? plotWidth : plotHeight;

		chart.plotBorderWidth = optionsChart.plotBorderWidth || 0;

		// Set boxes used for alignment
		chart.spacingBox = renderer.spacingBox = {
			x: spacing[3],
			y: spacing[0],
			width: chartWidth - spacing[3] - spacing[1],
			height: chartHeight - spacing[0] - spacing[2]
		};
		chart.plotBox = renderer.plotBox = {
			x: plotLeft,
			y: plotTop,
			width: plotWidth,
			height: plotHeight
		};

		plotBorderWidth = 2 * mathFloor(chart.plotBorderWidth / 2);
		clipX = mathCeil(mathMax(plotBorderWidth, clipOffset[3]) / 2);
		clipY = mathCeil(mathMax(plotBorderWidth, clipOffset[0]) / 2);
		chart.clipBox = {
			x: clipX,
			y: clipY,
			width: mathFloor(chart.plotSizeX - mathMax(plotBorderWidth, clipOffset[1]) / 2 - clipX),
			height: mathMax(0, mathFloor(chart.plotSizeY - mathMax(plotBorderWidth, clipOffset[2]) / 2 - clipY))
		};

		if (!skipAxes) {
			each(chart.axes, function (axis) {
				axis.setAxisSize();
				axis.setAxisTranslation();
			});
		}
	},

	/**
	 * Initial margins before auto size margins are applied
	 */
	resetMargins: function () {
		var chart = this;

		each(marginNames, function (m, side) {
			chart[m] = pick(chart.margin[side], chart.spacing[side]);
		});
		chart.axisOffset = [0, 0, 0, 0]; // top, right, bottom, left
		chart.clipOffset = [0, 0, 0, 0];
	},

	/**
	 * Draw the borders and backgrounds for chart and plot area
	 */
	drawChartBox: function () {
		var chart = this,
			optionsChart = chart.options.chart,
			renderer = chart.renderer,
			chartWidth = chart.chartWidth,
			chartHeight = chart.chartHeight,
			chartBackground = chart.chartBackground,
			plotBackground = chart.plotBackground,
			plotBorder = chart.plotBorder,
			plotBGImage = chart.plotBGImage,
			chartBorderWidth = optionsChart.borderWidth || 0,
			chartBackgroundColor = optionsChart.backgroundColor,
			plotBackgroundColor = optionsChart.plotBackgroundColor,
			plotBackgroundImage = optionsChart.plotBackgroundImage,
			plotBorderWidth = optionsChart.plotBorderWidth || 0,
			mgn,
			bgAttr,
			plotLeft = chart.plotLeft,
			plotTop = chart.plotTop,
			plotWidth = chart.plotWidth,
			plotHeight = chart.plotHeight,
			plotBox = chart.plotBox,
			clipRect = chart.clipRect,
			clipBox = chart.clipBox;

		// Chart area
		mgn = chartBorderWidth + (optionsChart.shadow ? 8 : 0);

		if (chartBorderWidth || chartBackgroundColor) {
			if (!chartBackground) {

				bgAttr = {
					fill: chartBackgroundColor || NONE
				};
				if (chartBorderWidth) { // #980
					bgAttr.stroke = optionsChart.borderColor;
					bgAttr['stroke-width'] = chartBorderWidth;
				}
				chart.chartBackground = renderer.rect(mgn / 2, mgn / 2, chartWidth - mgn, chartHeight - mgn,
						optionsChart.borderRadius, chartBorderWidth)
					.attr(bgAttr)
					.addClass(PREFIX + 'background')
					.add()
					.shadow(optionsChart.shadow);

			} else { // resize
				chartBackground.animate(
					chartBackground.crisp({ width: chartWidth - mgn, height: chartHeight - mgn })
				);
			}
		}


		// Plot background
		if (plotBackgroundColor) {
			if (!plotBackground) {
				chart.plotBackground = renderer.rect(plotLeft, plotTop, plotWidth, plotHeight, 0)
					.attr({
						fill: plotBackgroundColor
					})
					.add()
					.shadow(optionsChart.plotShadow);
			} else {
				plotBackground.animate(plotBox);
			}
		}
		if (plotBackgroundImage) {
			if (!plotBGImage) {
				chart.plotBGImage = renderer.image(plotBackgroundImage, plotLeft, plotTop, plotWidth, plotHeight)
					.add();
			} else {
				plotBGImage.animate(plotBox);
			}
		}

		// Plot clip
		if (!clipRect) {
			chart.clipRect = renderer.clipRect(clipBox);
		} else {
			clipRect.animate({
				width: clipBox.width,
				height: clipBox.height
			});
		}

		// Plot area border
		if (plotBorderWidth) {
			if (!plotBorder) {
				chart.plotBorder = renderer.rect(plotLeft, plotTop, plotWidth, plotHeight, 0, -plotBorderWidth)
					.attr({
						stroke: optionsChart.plotBorderColor,
						'stroke-width': plotBorderWidth,
						fill: NONE,
						zIndex: 1
					})
					.add();
			} else {
				plotBorder.animate(
					plotBorder.crisp({ x: plotLeft, y: plotTop, width: plotWidth, height: plotHeight, strokeWidth: -plotBorderWidth }) //#3282 plotBorder should be negative
				);
			}
		}

		// reset
		chart.isDirtyBox = false;
	},

	/**
	 * Detect whether a certain chart property is needed based on inspecting its options
	 * and series. This mainly applies to the chart.invert property, and in extensions to
	 * the chart.angular and chart.polar properties.
	 */
	propFromSeries: function () {
		var chart = this,
			optionsChart = chart.options.chart,
			klass,
			seriesOptions = chart.options.series,
			i,
			value;


		each(['inverted', 'angular', 'polar'], function (key) {

			// The default series type's class
			klass = seriesTypes[optionsChart.type || optionsChart.defaultSeriesType];

			// Get the value from available chart-wide properties
			value = (
				chart[key] || // 1. it is set before
				optionsChart[key] || // 2. it is set in the options
				(klass && klass.prototype[key]) // 3. it's default series class requires it
			);

			// 4. Check if any the chart's series require it
			i = seriesOptions && seriesOptions.length;
			while (!value && i--) {
				klass = seriesTypes[seriesOptions[i].type];
				if (klass && klass.prototype[key]) {
					value = true;
				}
			}

			// Set the chart property
			chart[key] = value;
		});

	},

	/**
	 * Link two or more series together. This is done initially from Chart.render,
	 * and after Chart.addSeries and Series.remove.
	 */
	linkSeries: function () {
		var chart = this,
			chartSeries = chart.series;

		// Reset links
		each(chartSeries, function (series) {
			series.linkedSeries.length = 0;
		});

		// Apply new links
		each(chartSeries, function (series) {
			var linkedTo = series.options.linkedTo;
			if (isString(linkedTo)) {
				if (linkedTo === ':previous') {
					linkedTo = chart.series[series.index - 1];
				} else {
					linkedTo = chart.get(linkedTo);
				}
				if (linkedTo) {
					linkedTo.linkedSeries.push(series);
					series.linkedParent = linkedTo;
					series.visible = pick(series.options.visible, linkedTo.options.visible, series.visible); // #3879
				}
			}
		});
	},

	/**
	 * Render series for the chart
	 */
	renderSeries: function () {
		each(this.series, function (serie) {
			serie.translate();
			serie.render();
		});
	},

	/**
	 * Render labels for the chart
	 */
	renderLabels: function () {
		var chart = this,
			labels = chart.options.labels;
		if (labels.items) {
			each(labels.items, function (label) {
				var style = extend(labels.style, label.style),
					x = pInt(style.left) + chart.plotLeft,
					y = pInt(style.top) + chart.plotTop + 12;

				// delete to prevent rewriting in IE
				delete style.left;
				delete style.top;

				chart.renderer.text(
					label.html,
					x,
					y
				)
				.attr({ zIndex: 2 })
				.css(style)
				.add();

			});
		}
	},

	/**
	 * Render all graphics for the chart
	 */
	render: function () {
		var chart = this,
			axes = chart.axes,
			renderer = chart.renderer,
			options = chart.options,
			tempWidth,
			tempHeight,
			redoHorizontal,
			redoVertical;

		// Title
		chart.setTitle();


		// Legend
		chart.legend = new Legend(chart, options.legend);

		// Get stacks
		if (chart.getStacks) {
			chart.getStacks();
		}

		// Get chart margins
		chart.getMargins(true);
		chart.setChartSize();

		// Record preliminary dimensions for later comparison
		tempWidth = chart.plotWidth;
		tempHeight = chart.plotHeight = chart.plotHeight - 21; // 21 is the most common correction for X axis labels

		// Get margins by pre-rendering axes
		each(axes, function (axis) {
			axis.setScale();
		});
		chart.getAxisMargins();

		// If the plot area size has changed significantly, calculate tick positions again
		redoHorizontal = tempWidth / chart.plotWidth > 1.1;
		redoVertical = tempHeight / chart.plotHeight > 1.05; // Height is more sensitive

		if (redoHorizontal || redoVertical) {

			chart.maxTicks = null; // reset for second pass
			each(axes, function (axis) {
				if ((axis.horiz && redoHorizontal) || (!axis.horiz && redoVertical)) {
					axis.setTickInterval(true); // update to reflect the new margins
				}
			});
			chart.getMargins(); // second pass to check for new labels
		}

		// Draw the borders and backgrounds
		chart.drawChartBox();


		// Axes
		if (chart.hasCartesianSeries) {
			each(axes, function (axis) {
				if (axis.visible) {
					axis.render();
				}
			});
		}

		// The series
		if (!chart.seriesGroup) {
			chart.seriesGroup = renderer.g('series-group')
				.attr({ zIndex: 3 })
				.add();
		}
		chart.renderSeries();

		// Labels
		chart.renderLabels();

		// Credits
		chart.showCredits(options.credits);

		// Set flag
		chart.hasRendered = true;

	},

	/**
	 * Show chart credits based on config options
	 */
	showCredits: function (credits) {
		if (credits.enabled && !this.credits) {
			this.credits = this.renderer.text(
				credits.text,
				0,
				0
			)
			.on('click', function () {
				if (credits.href) {
					location.href = credits.href;
				}
			})
			.attr({
				align: credits.position.align,
				zIndex: 8
			})
			.css(credits.style)
			.add()
			.align(credits.position);
		}
	},

	/**
	 * Clean up memory usage
	 */
	destroy: function () {
		var chart = this,
			axes = chart.axes,
			series = chart.series,
			container = chart.container,
			i,
			parentNode = container && container.parentNode;

		// fire the chart.destoy event
		fireEvent(chart, 'destroy');

		// Delete the chart from charts lookup array
		charts[chart.index] = UNDEFINED;
		chartCount--;
		chart.renderTo.removeAttribute('data-highcharts-chart');

		// remove events
		removeEvent(chart);

		// ==== Destroy collections:
		// Destroy axes
		i = axes.length;
		while (i--) {
			axes[i] = axes[i].destroy();
		}

		// Destroy each series
		i = series.length;
		while (i--) {
			series[i] = series[i].destroy();
		}

		// ==== Destroy chart properties:
		each(['title', 'subtitle', 'chartBackground', 'plotBackground', 'plotBGImage',
				'plotBorder', 'seriesGroup', 'clipRect', 'credits', 'pointer', 'scroller',
				'rangeSelector', 'legend', 'resetZoomButton', 'tooltip', 'renderer'], function (name) {
			var prop = chart[name];

			if (prop && prop.destroy) {
				chart[name] = prop.destroy();
			}
		});

		// remove container and all SVG
		if (container) { // can break in IE when destroyed before finished loading
			container.innerHTML = '';
			removeEvent(container);
			if (parentNode) {
				discardElement(container);
			}

		}

		// clean it all up
		for (i in chart) {
			delete chart[i];
		}

	},


	/**
	 * VML namespaces can't be added until after complete. Listening
	 * for Perini's doScroll hack is not enough.
	 */
	isReadyToRender: function () {
		var chart = this;

		// Note: win == win.top is required
		if ((!hasSVG && (win == win.top && doc.readyState !== 'complete')) || (useCanVG && !win.canvg)) { // eslint-disable-line eqeqeq
			if (useCanVG) {
				// Delay rendering until canvg library is downloaded and ready
				CanVGController.push(function () {
					chart.firstRender();
				}, chart.options.global.canvasToolsURL);
			} else {
				doc.attachEvent('onreadystatechange', function () {
					doc.detachEvent('onreadystatechange', chart.firstRender);
					if (doc.readyState === 'complete') {
						chart.firstRender();
					}
				});
			}
			return false;
		}
		return true;
	},

	/**
	 * Prepare for first rendering after all data are loaded
	 */
	firstRender: function () {
		var chart = this,
			options = chart.options,
			callback = chart.callback;

		// Check whether the chart is ready to render
		if (!chart.isReadyToRender()) {
			return;
		}

		// Create the container
		chart.getContainer();

		// Run an early event after the container and renderer are established
		fireEvent(chart, 'init');


		chart.resetMargins();
		chart.setChartSize();

		// Set the common chart properties (mainly invert) from the given series
		chart.propFromSeries();

		// get axes
		chart.getAxes();

		// Initialize the series
		each(options.series || [], function (serieOptions) {
			chart.initSeries(serieOptions);
		});

		chart.linkSeries();

		// Run an event after axes and series are initialized, but before render. At this stage,
		// the series data is indexed and cached in the xData and yData arrays, so we can access
		// those before rendering. Used in Highstock.
		fireEvent(chart, 'beforeRender');

		// depends on inverted and on margins being set
		if (Highcharts.Pointer) {
			chart.pointer = new Pointer(chart, options);
		}

		chart.render();

		// add canvas
		chart.renderer.draw();
		// run callbacks
		if (callback) {
			callback.apply(chart, [chart]);
		}
		each(chart.callbacks, function (fn) {
			if (chart.index !== UNDEFINED) { // Chart destroyed in its own callback (#3600)
				fn.apply(chart, [chart]);
			}
		});

		// Fire the load event
		fireEvent(chart, 'load');

		// If the chart was rendered outside the top container, put it back in (#3679)
		chart.cloneRenderTo(true);

	},

	/**
	* Creates arrays for spacing and margin from given options.
	*/
	splashArray: function (target, options) {
		var oVar = options[target],
			tArray = isObject(oVar) ? oVar : [oVar, oVar, oVar, oVar];

		return [pick(options[target + 'Top'], tArray[0]),
				pick(options[target + 'Right'], tArray[1]),
				pick(options[target + 'Bottom'], tArray[2]),
				pick(options[target + 'Left'], tArray[3])];
	}
}; // end Chart
