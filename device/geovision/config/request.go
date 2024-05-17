/*
DESCRIPTION
  request.go provides unexported functionality for creating and sending requests
  required to configure settings of the GeoVision camera.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/
*;q=0.8,application/signed-exchange;v=b3")
	req.Header.Set("Referer", "http://"+host+"/ssi.cgi/VideoSettingSub.htm?cam="+strconv.Itoa(int(s.ch)))
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Accept-Language", "en-GB,en-US;q=0.9,en;q=0.8")
	req.Header.Set("Cookie", "CLIENT_ID="+id)

	// NB: not capturing error, as we always get one here for some reason.
	// TODO: figure out why. Does not affect submission.
	c.Do(req)

	return nil
}
