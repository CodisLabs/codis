/*
 
 Highcharts funnel module

 (c) 2010-2014 Torstein Honsi

 License: www.highcharts.com/license
*/
(function(b){typeof module==="object"&&module.exports?module.exports=b:b(Highcharts)})(function(b){var q=b.getOptions(),w=q.plotOptions,r=b.seriesTypes,G=b.merge,E=function(){},B=b.each,F=b.pick;w.funnel=G(w.pie,{animation:!1,center:["50%","50%"],width:"90%",neckWidth:"30%",height:"100%",neckHeight:"25%",reversed:!1,dataLabels:{connectorWidth:1,connectorColor:"#606060"},size:!0,states:{select:{color:"#C0C0C0",borderColor:"#000000",shadow:!1}}});r.funnel=b.extendClass(r.pie,{type:"funnel",animate:E,
translate:function(){var a=function(k,a){return/%$/.test(k)?a*parseInt(k,10)/100:parseInt(k,10)},b=0,e=this.chart,c=this.options,f=c.reversed,o=c.ignoreHiddenPoint,g=e.plotWidth,h=e.plotHeight,q=0,e=c.center,i=a(e[0],g),x=a(e[1],h),r=a(c.width,g),l,s,d=a(c.height,h),t=a(c.neckWidth,g),C=a(c.neckHeight,h),u=x-d/2+d-C,a=this.data,y,z,w=c.dataLabels.position==="left"?1:0,A,m,D,p,j,v,n;this.getWidthAt=s=function(k){var a=x-d/2;return k>u||d===C?t:t+(r-t)*(1-(k-a)/(d-C))};this.getX=function(k,a){return i+
(a?-1:1)*(s(f?h-k:k)/2+c.dataLabels.distance)};this.center=[i,x,d];this.centerX=i;B(a,function(a){if(!o||a.visible!==!1)b+=a.y});B(a,function(a){n=null;z=b?a.y/b:0;m=x-d/2+q*d;j=m+z*d;l=s(m);A=i-l/2;D=A+l;l=s(j);p=i-l/2;v=p+l;m>u?(A=p=i-t/2,D=v=i+t/2):j>u&&(n=j,l=s(u),p=i-l/2,v=p+l,j=u);f&&(m=d-m,j=d-j,n=n?d-n:null);y=["M",A,m,"L",D,m,v,j];n&&y.push(v,n,p,n);y.push(p,j,"Z");a.shapeType="path";a.shapeArgs={d:y};a.percentage=z*100;a.plotX=i;a.plotY=(m+(n||j))/2;a.tooltipPos=[i,a.plotY];a.slice=E;a.half=
w;if(!o||a.visible!==!1)q+=z})},drawPoints:function(){var a=this,b=a.options,e=a.chart.renderer;B(a.data,function(c){var f=c.options,o=c.graphic,g=c.shapeArgs;o?o.animate(g):c.graphic=e.path(g).attr({fill:c.color,stroke:F(f.borderColor,b.borderColor),"stroke-width":F(f.borderWidth,b.borderWidth)}).add(a.group)})},sortByAngle:function(a){a.sort(function(a,b){return a.plotY-b.plotY})},drawDataLabels:function(){var a=this.data,b=this.options.dataLabels.distance,e,c,f,o=a.length,g,h;for(this.center[2]-=
2*b;o--;)f=a[o],c=(e=f.half)?1:-1,h=f.plotY,g=this.getX(h,e),f.labelPos=[0,h,g+(b-5)*c,h,g+b*c,h,e?"right":"left",0];r.pie.prototype.drawDataLabels.call(this)}});q.plotOptions.pyramid=b.merge(q.plotOptions.funnel,{neckWidth:"0%",neckHeight:"0%",reversed:!0});b.seriesTypes.pyramid=b.extendClass(b.seriesTypes.funnel,{type:"pyramid"})});
