/**
 * The class for stack items
 */
function StackItem(axis, options, isNegative, x, stackOption) {

	var inverted = axis.chart.inverted;

	this.axis = axis;

	// Tells if the stack is negative
	this.isNegative = isNegative;

	// Save the options to be able to style the label
	this.options = options;

	// Save the x value to be able to position the label later
	this.x = x;

	// Initialize total value
	this.total = null;

	// This will keep each points' extremes stored by series.index and point index
	this.points = {};

	// Save the stack option on the series configuration object, and whether to treat it as percent
	this.stack = stackOption;

	// The align options and text align varies on whether the stack is negative and
	// if the chart is inverted or not.
	// First test the user supplied value, then use the dynamic.
	this.alignOptions = {
		align: options.align || (inverted ? (isNegative ? 'left' : 'right') : 'center'),
		verticalAlign: options.verticalAlign || (inverted ? 'middle' : (isNegative ? 'bottom' : 'top')),
		y: pick(options.y, inverted ? 4 : (isNegative ? 14 : -6)),
		x: pick(options.x, inverted ? (isNegative ? -6 : 6) : 0)
	};

	this.textAlign = options.textAlign || (inverted ? (isNegative ? 'right' : 'left') : 'center');
}

StackItem.prototype = {
	destroy: function () {
		destroyObjectProperties(this, this.axis);
	},

	/**
	 * Renders the stack total label and adds it to the stack label group.
	 */
	render: function (group) {
		var options = this.options,
			formatOption = options.format,
			str = formatOption ?
				format(formatOption, this) :
				options.formatter.call(this);  // format the text in the label

		// Change the text to reflect the new total and set visibility to hidden in case the serie is hidden
		if (this.label) {
			this.label.attr({ text: str, visibility: 'hidden' });
		// Create new label
		} else {
			this.label =
				this.axis.chart.renderer.text(str, null, null, options.useHTML)		// dummy positions, actual position updated with setOffset method in columnseries
					.css(options.style)				// apply style
					.attr({
						align: this.textAlign,				// fix the text-anchor
						rotation: options.rotation,	// rotation
						visibility: HIDDEN					// hidden until setOffset is called
					})
					.add(group);							// add to the labels-group
		}
	},

	/**
	 * Sets the offset that the stack has from the x value and repositions the label.
	 */
	setOffset: function (xOffset, xWidth) {
		var stackItem = this,
			axis = stackItem.axis,
			chart = axis.chart,
			inverted = chart.inverted,
			reversed = axis.reversed,
			neg = (this.isNegative && !reversed) || (!this.isNegative && reversed), // #4056
			y = axis.translate(axis.usePercentage ? 100 : this.total, 0, 0, 0, 1), // stack value translated mapped to chart coordinates
			yZero = axis.translate(0),						// stack origin
			h = mathAbs(y - yZero),							// stack height
			x = chart.xAxis[0].translate(this.x) + xOffset,	// stack x position
			plotHeight = chart.plotHeight,
			stackBox = {	// this is the box for the complete stack
				x: inverted ? (neg ? y : y - h) : x,
				y: inverted ? plotHeight - x - xWidth : (neg ? (plotHeight - y - h) : plotHeight - y),
				width: inverted ? h : xWidth,
				height: inverted ? xWidth : h
			},
			label = this.label,
			alignAttr;

		if (label) {
			label.align(this.alignOptions, null, stackBox);	// align the label to the box

			// Set visibility (#678)
			alignAttr = label.alignAttr;
			label[this.options.crop === false || chart.isInsidePlot(alignAttr.x, alignAttr.y) ? 'show' : 'hide'](true);
		}
	}
};

/**
 * Generate stacks for each series and calculate stacks total values
 */
Chart.prototype.getStacks = function () {
	var chart = this;

	// reset stacks for each yAxis
	each(chart.yAxis, function (axis) {
		if (axis.stacks && axis.hasVisibleSeries) {
			axis.oldStacks = axis.stacks;
		}
	});

	each(chart.series, function (series) {
		if (series.options.stacking && (series.visible === true || chart.options.chart.ignoreHiddenSeries === false)) {
			series.stackKey = series.type + pick(series.options.stack, '');
		}
	});
};


// Stacking methods defined on the Axis prototype

/**
 * Build the stacks from top down
 */
Axis.prototype.buildStacks = function () {
	var series = this.series,
		reversedStacks = pick(this.options.reversedStacks, true),
		i = series.length;
	if (!this.isXAxis) {
		this.usePercentage = false;
		while (i--) {
			series[reversedStacks ? i : series.length - i - 1].setStackedPoints();
		}
		// Loop up again to compute percent stack
		if (this.usePercentage) {
			for (i = 0; i < series.length; i++) {
				series[i].setPercentStacks();
			}
		}
	}
};

Axis.prototype.renderStackTotals = function () {
	var axis = this,
		chart = axis.chart,
		renderer = chart.renderer,
		stacks = axis.stacks,
		stackKey,
		oneStack,
		stackCategory,
		stackTotalGroup = axis.stackTotalGroup;

	// Create a separate group for the stack total labels
	if (!stackTotalGroup) {
		axis.stackTotalGroup = stackTotalGroup =
			renderer.g('stack-labels')
				.attr({
					visibility: VISIBLE,
					zIndex: 6
				})
				.add();
	}

	// plotLeft/Top will change when y axis gets wider so we need to translate the
	// stackTotalGroup at every render call. See bug #506 and #516
	stackTotalGroup.translate(chart.plotLeft, chart.plotTop);

	// Render each stack total
	for (stackKey in stacks) {
		oneStack = stacks[stackKey];
		for (stackCategory in oneStack) {
			oneStack[stackCategory].render(stackTotalGroup);
		}
	}
};

/**
 * Set all the stacks to initial states and destroy unused ones.
 */
Axis.prototype.resetStacks = function () {
	var stacks = this.stacks,
		type,
		i;
	if (!this.isXAxis) {
		for (type in stacks) {
			for (i in stacks[type]) {

				// Clean up memory after point deletion (#1044, #4320)
				if (stacks[type][i].touched < this.stacksTouched) {
					stacks[type][i].destroy();
					delete stacks[type][i];

				// Reset stacks
				} else {
					stacks[type][i].total = null;
					stacks[type][i].cum = 0;
				}
			}
		}
	}
};

Axis.prototype.cleanStacks = function () {
	var stacks, type, i;

	if (!this.isXAxis) {
		if (this.oldStacks) {
			stacks = this.stacks = this.oldStacks;
		}

		// reset stacks
		for (type in stacks) {
			for (i in stacks[type]) {
				stacks[type][i].cum = stacks[type][i].total;
			}
		}
	}
};


// Stacking methods defnied for Series prototype

/**
 * Adds series' points value to corresponding stack
 */
Series.prototype.setStackedPoints = function () {
	if (!this.options.stacking || (this.visible !== true && this.chart.options.chart.ignoreHiddenSeries !== false)) {
		return;
	}

	var series = this,
		xData = series.processedXData,
		yData = series.processedYData,
		stackedYData = [],
		yDataLength = yData.length,
		seriesOptions = series.options,
		threshold = seriesOptions.threshold,
		stackThreshold = seriesOptions.startFromThreshold ? threshold : 0,
		stackOption = seriesOptions.stack,
		stacking = seriesOptions.stacking,
		stackKey = series.stackKey,
		negKey = '-' + stackKey,
		negStacks = series.negStacks,
		yAxis = series.yAxis,
		stacks = yAxis.stacks,
		oldStacks = yAxis.oldStacks,
		stackIndicator,
		isNegative,
		stack,
		other,
		key,
		pointKey,
		i,
		x,
		y;


	yAxis.stacksTouched += 1;

	// loop over the non-null y values and read them into a local array
	for (i = 0; i < yDataLength; i++) {
		x = xData[i];
		y = yData[i];
		stackIndicator = series.getStackIndicator(stackIndicator, x, series.index);
		pointKey = stackIndicator.key;
		// Read stacked values into a stack based on the x value,
		// the sign of y and the stack key. Stacking is also handled for null values (#739)
		isNegative = negStacks && y < (stackThreshold ? 0 : threshold);
		key = isNegative ? negKey : stackKey;

		// Create empty object for this stack if it doesn't exist yet
		if (!stacks[key]) {
			stacks[key] = {};
		}

		// Initialize StackItem for this x
		if (!stacks[key][x]) {
			if (oldStacks[key] && oldStacks[key][x]) {
				stacks[key][x] = oldStacks[key][x];
				stacks[key][x].total = null;
			} else {
				stacks[key][x] = new StackItem(yAxis, yAxis.options.stackLabels, isNegative, x, stackOption);
			}
		}

		// If the StackItem doesn't exist, create it first
		stack = stacks[key][x];
		stack.points[pointKey] = [pick(stack.cum, stackThreshold)];
		stack.touched = yAxis.stacksTouched;

		// In area charts, if there are multiple points on the same X value, let the 
		// area fill the full span of those points
		if (stackIndicator.index > 0 && series.singleStacks === false) {
			stack.points[pointKey][0] = stack.points[series.index + ',' + x + ',0'][0];
		}

		// Add value to the stack total
		if (stacking === 'percent') {

			// Percent stacked column, totals are the same for the positive and negative stacks
			other = isNegative ? stackKey : negKey;
			if (negStacks && stacks[other] && stacks[other][x]) {
				other = stacks[other][x];
				stack.total = other.total = mathMax(other.total, stack.total) + mathAbs(y) || 0;

			// Percent stacked areas
			} else {
				stack.total = correctFloat(stack.total + (mathAbs(y) || 0));
			}
		} else {
			stack.total = correctFloat(stack.total + (y || 0));
		}

		stack.cum = pick(stack.cum, stackThreshold) + (y || 0);

		stack.points[pointKey].push(stack.cum);
		stackedYData[i] = stack.cum;

	}

	if (stacking === 'percent') {
		yAxis.usePercentage = true;
	}

	this.stackedYData = stackedYData; // To be used in getExtremes

	// Reset old stacks
	yAxis.oldStacks = {};
};

/**
 * Iterate over all stacks and compute the absolute values to percent
 */
Series.prototype.setPercentStacks = function () {
	var series = this,
		stackKey = series.stackKey,
		stacks = series.yAxis.stacks,
		processedXData = series.processedXData,
		stackIndicator;

	each([stackKey, '-' + stackKey], function (key) {
		var i = processedXData.length,
			x,
			stack,
			pointExtremes,
			totalFactor;

		while (i--) {
			x = processedXData[i];
			stackIndicator = series.getStackIndicator(stackIndicator, x, series.index);
			stack = stacks[key] && stacks[key][x];
			pointExtremes = stack && stack.points[stackIndicator.key];
			if (pointExtremes) {
				totalFactor = stack.total ? 100 / stack.total : 0;
				pointExtremes[0] = correctFloat(pointExtremes[0] * totalFactor); // Y bottom value
				pointExtremes[1] = correctFloat(pointExtremes[1] * totalFactor); // Y value
				series.stackedYData[i] = pointExtremes[1];
			}
		}
	});
};

/**
* Get stack indicator, according to it's x-value, to determine points with the same x-value
*/
Series.prototype.getStackIndicator = function (stackIndicator, x, index) {
	if (!defined(stackIndicator) || stackIndicator.x !== x) {
		stackIndicator = {
			x: x,
			index: 0
		};
	} else {
		stackIndicator.index++;
	}

	stackIndicator.key = [index, x, stackIndicator.index].join(',');

	return stackIndicator;
};

