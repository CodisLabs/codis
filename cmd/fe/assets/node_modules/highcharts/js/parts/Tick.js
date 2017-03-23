/**
 * The Tick class
 */
function Tick(axis, pos, type, noLabel) {
	this.axis = axis;
	this.pos = pos;
	this.type = type || '';
	this.isNew = true;

	if (!type && !noLabel) {
		this.addLabel();
	}
}

Tick.prototype = {
	/**
	 * Write the tick label
	 */
	addLabel: function () {
		var tick = this,
			axis = tick.axis,
			options = axis.options,
			chart = axis.chart,
			categories = axis.categories,
			names = axis.names,
			pos = tick.pos,
			labelOptions = options.labels,
			str,
			tickPositions = axis.tickPositions,
			isFirst = pos === tickPositions[0],
			isLast = pos === tickPositions[tickPositions.length - 1],
			value = categories ?
				pick(categories[pos], names[pos], pos) :
				pos,
			label = tick.label,
			tickPositionInfo = tickPositions.info,
			dateTimeLabelFormat;

		// Set the datetime label format. If a higher rank is set for this position, use that. If not,
		// use the general format.
		if (axis.isDatetimeAxis && tickPositionInfo) {
			dateTimeLabelFormat = options.dateTimeLabelFormats[tickPositionInfo.higherRanks[pos] || tickPositionInfo.unitName];
		}
		// set properties for access in render method
		tick.isFirst = isFirst;
		tick.isLast = isLast;

		// get the string
		str = axis.labelFormatter.call({
			axis: axis,
			chart: chart,
			isFirst: isFirst,
			isLast: isLast,
			dateTimeLabelFormat: dateTimeLabelFormat,
			value: axis.isLog ? correctFloat(lin2log(value)) : value
		});

		// prepare CSS
		//css = width && { width: mathMax(1, mathRound(width - 2 * (labelOptions.padding || 10))) + PX };

		// first call
		if (!defined(label)) {

			tick.label = label =
				defined(str) && labelOptions.enabled ?
					chart.renderer.text(
							str,
							0,
							0,
							labelOptions.useHTML
						)
						//.attr(attr)
						// without position absolute, IE export sometimes is wrong
						.css(merge(labelOptions.style))
						.add(axis.labelGroup) :
					null;
			tick.labelLength = label && label.getBBox().width; // Un-rotated length
			tick.rotation = 0; // Base value to detect change for new calls to getBBox

		// update
		} else if (label) {
			label.attr({ text: str });
		}
	},

	/**
	 * Get the offset height or width of the label
	 */
	getLabelSize: function () {
		return this.label ?
			this.label.getBBox()[this.axis.horiz ? 'height' : 'width'] :
			0;
	},

	/**
	 * Handle the label overflow by adjusting the labels to the left and right edge, or
	 * hide them if they collide into the neighbour label.
	 */
	handleOverflow: function (xy) {
		var axis = this.axis,
			pxPos = xy.x,
			chartWidth = axis.chart.chartWidth,
			spacing = axis.chart.spacing,
			leftBound = pick(axis.labelLeft, mathMin(axis.pos, spacing[3])),
			rightBound = pick(axis.labelRight, mathMax(axis.pos + axis.len, chartWidth - spacing[1])),
			label = this.label,
			rotation = this.rotation,
			factor = { left: 0, center: 0.5, right: 1 }[axis.labelAlign],
			labelWidth = label.getBBox().width,
			slotWidth = axis.slotWidth,
			xCorrection = factor,
			goRight = 1,
			leftPos,
			rightPos,
			textWidth,
			css = {};

		// Check if the label overshoots the chart spacing box. If it does, move it.
		// If it now overshoots the slotWidth, add ellipsis.
		if (!rotation) {
			leftPos = pxPos - factor * labelWidth;
			rightPos = pxPos + (1 - factor) * labelWidth;

			if (leftPos < leftBound) {
				slotWidth = xy.x + slotWidth * (1 - factor) - leftBound;
			} else if (rightPos > rightBound) {
				slotWidth = rightBound - xy.x + slotWidth * factor;
				goRight = -1;
			}

			slotWidth = mathMin(axis.slotWidth, slotWidth); // #4177
			if (slotWidth < axis.slotWidth && axis.labelAlign === 'center') {
				xy.x += goRight * (axis.slotWidth - slotWidth - xCorrection * (axis.slotWidth - mathMin(labelWidth, slotWidth)));
			}
			// If the label width exceeds the available space, set a text width to be
			// picked up below. Also, if a width has been set before, we need to set a new
			// one because the reported labelWidth will be limited by the box (#3938).
			if (labelWidth > slotWidth || (axis.autoRotation && label.styles.width)) {
				textWidth = slotWidth;
			}

		// Add ellipsis to prevent rotated labels to be clipped against the edge of the chart
		} else if (rotation < 0 && pxPos - factor * labelWidth < leftBound) {
			textWidth = mathRound(pxPos / mathCos(rotation * deg2rad) - leftBound);
		} else if (rotation > 0 && pxPos + factor * labelWidth > rightBound) {
			textWidth = mathRound((chartWidth - pxPos) / mathCos(rotation * deg2rad));
		}

		if (textWidth) {
			css.width = textWidth;
			if (!axis.options.labels.style.textOverflow) {
				css.textOverflow = 'ellipsis';
			}
			label.css(css);
		}
	},

	/**
	 * Get the x and y position for ticks and labels
	 */
	getPosition: function (horiz, pos, tickmarkOffset, old) {
		var axis = this.axis,
			chart = axis.chart,
			cHeight = (old && chart.oldChartHeight) || chart.chartHeight;

		return {
			x: horiz ?
				axis.translate(pos + tickmarkOffset, null, null, old) + axis.transB :
				axis.left + axis.offset + (axis.opposite ? ((old && chart.oldChartWidth) || chart.chartWidth) - axis.right - axis.left : 0),

			y: horiz ?
				cHeight - axis.bottom + axis.offset - (axis.opposite ? axis.height : 0) :
				cHeight - axis.translate(pos + tickmarkOffset, null, null, old) - axis.transB
		};

	},

	/**
	 * Get the x, y position of the tick label
	 */
	getLabelPosition: function (x, y, label, horiz, labelOptions, tickmarkOffset, index, step) {
		var axis = this.axis,
			transA = axis.transA,
			reversed = axis.reversed,
			staggerLines = axis.staggerLines,
			rotCorr = axis.tickRotCorr || { x: 0, y: 0 },
			yOffset = labelOptions.y,
			line;

		if (!defined(yOffset)) {
			yOffset = axis.side === 2 ? 
				rotCorr.y + 8 :
				// #3140, #3140
				yOffset = mathCos(label.rotation * deg2rad) * (rotCorr.y - label.getBBox(false, 0).height / 2);
		}

		x = x + labelOptions.x + rotCorr.x - (tickmarkOffset && horiz ?
			tickmarkOffset * transA * (reversed ? -1 : 1) : 0);
		y = y + yOffset - (tickmarkOffset && !horiz ?
			tickmarkOffset * transA * (reversed ? 1 : -1) : 0);

		// Correct for staggered labels
		if (staggerLines) {
			line = (index / (step || 1) % staggerLines);
			if (axis.opposite) {
				line = staggerLines - line - 1;
			}
			y += line * (axis.labelOffset / staggerLines);
		}

		return {
			x: x,
			y: mathRound(y)
		};
	},

	/**
	 * Extendible method to return the path of the marker
	 */
	getMarkPath: function (x, y, tickLength, tickWidth, horiz, renderer) {
		return renderer.crispLine([
				M,
				x,
				y,
				L,
				x + (horiz ? 0 : -tickLength),
				y + (horiz ? tickLength : 0)
			], tickWidth);
	},

	/**
	 * Put everything in place
	 *
	 * @param index {Number}
	 * @param old {Boolean} Use old coordinates to prepare an animation into new position
	 */
	render: function (index, old, opacity) {
		var tick = this,
			axis = tick.axis,
			options = axis.options,
			chart = axis.chart,
			renderer = chart.renderer,
			horiz = axis.horiz,
			type = tick.type,
			label = tick.label,
			pos = tick.pos,
			labelOptions = options.labels,
			gridLine = tick.gridLine,
			gridPrefix = type ? type + 'Grid' : 'grid',
			tickPrefix = type ? type + 'Tick' : 'tick',
			gridLineWidth = options[gridPrefix + 'LineWidth'],
			gridLineColor = options[gridPrefix + 'LineColor'],
			dashStyle = options[gridPrefix + 'LineDashStyle'],
			tickLength = options[tickPrefix + 'Length'],
			tickWidth = pick(options[tickPrefix + 'Width'], !type && axis.isXAxis ? 1 : 0), // X axis defaults to 1
			tickColor = options[tickPrefix + 'Color'],
			tickPosition = options[tickPrefix + 'Position'],
			gridLinePath,
			mark = tick.mark,
			markPath,
			step = /*axis.labelStep || */labelOptions.step,
			attribs,
			show = true,
			tickmarkOffset = axis.tickmarkOffset,
			xy = tick.getPosition(horiz, pos, tickmarkOffset, old),
			x = xy.x,
			y = xy.y,
			reverseCrisp = ((horiz && x === axis.pos + axis.len) || (!horiz && y === axis.pos)) ? -1 : 1; // #1480, #1687

		opacity = pick(opacity, 1);
		this.isActive = true;

		// create the grid line
		if (gridLineWidth) {
			gridLinePath = axis.getPlotLinePath(pos + tickmarkOffset, gridLineWidth * reverseCrisp, old, true);

			if (gridLine === UNDEFINED) {
				attribs = {
					stroke: gridLineColor,
					'stroke-width': gridLineWidth
				};
				if (dashStyle) {
					attribs.dashstyle = dashStyle;
				}
				if (!type) {
					attribs.zIndex = 1;
				}
				if (old) {
					attribs.opacity = 0;
				}
				tick.gridLine = gridLine =
					gridLineWidth ?
						renderer.path(gridLinePath)
							.attr(attribs).add(axis.gridGroup) :
						null;
			}

			// If the parameter 'old' is set, the current call will be followed
			// by another call, therefore do not do any animations this time
			if (!old && gridLine && gridLinePath) {
				gridLine[tick.isNew ? 'attr' : 'animate']({
					d: gridLinePath,
					opacity: opacity
				});
			}
		}

		// create the tick mark
		if (tickWidth && tickLength) {

			// negate the length
			if (tickPosition === 'inside') {
				tickLength = -tickLength;
			}
			if (axis.opposite) {
				tickLength = -tickLength;
			}

			markPath = tick.getMarkPath(x, y, tickLength, tickWidth * reverseCrisp, horiz, renderer);
			if (mark) { // updating
				mark.animate({
					d: markPath,
					opacity: opacity
				});
			} else { // first time
				tick.mark = renderer.path(
					markPath
				).attr({
					stroke: tickColor,
					'stroke-width': tickWidth,
					opacity: opacity
				}).add(axis.axisGroup);
			}
		}

		// the label is created on init - now move it into place
		if (label && !isNaN(x)) {
			label.xy = xy = tick.getLabelPosition(x, y, label, horiz, labelOptions, tickmarkOffset, index, step);

			// Apply show first and show last. If the tick is both first and last, it is
			// a single centered tick, in which case we show the label anyway (#2100).
			if ((tick.isFirst && !tick.isLast && !pick(options.showFirstLabel, 1)) ||
					(tick.isLast && !tick.isFirst && !pick(options.showLastLabel, 1))) {
				show = false;

			// Handle label overflow and show or hide accordingly
			} else if (horiz && !axis.isRadial && !labelOptions.step && !labelOptions.rotation && !old && opacity !== 0) {
				tick.handleOverflow(xy);
			}

			// apply step
			if (step && index % step) {
				// show those indices dividable by step
				show = false;
			}

			// Set the new position, and show or hide
			if (show && !isNaN(xy.y)) {
				xy.opacity = opacity;
				label[tick.isNew ? 'attr' : 'animate'](xy);
				tick.isNew = false;
			} else {
				label.attr('y', -9999); // #1338
			}
		}
	},

	/**
	 * Destructor for the tick prototype
	 */
	destroy: function () {
		destroyObjectProperties(this, this.axis);
	}
};

