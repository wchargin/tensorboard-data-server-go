use byteorder::{ByteOrder, LittleEndian};
use std::io::{self, Read, Write};

use crate::masked_crc::{compute_crc, MaskedCrc};

const LENGTH_CRC_OFFSET: usize = 8;
const DATA_OFFSET: usize = LENGTH_CRC_OFFSET + 4;
const HEADER_LENGTH: usize = DATA_OFFSET;
const FOOTER_LENGTH: usize = 4;

// From TensorFlow `record_writer.cc` comments:
// Format of a single record:
//  uint64    length
//  uint32    masked crc of length
//  byte      data[length]
//  uint32    masked crc of data
pub struct TfRecordState {
    /// TFRecord header: little-endian u64 length, u32 length-CRC.
    header: Vec<u8>,
    /// Everything past the header in the TFRecord: the data buffer, plus a little-endian u32 CRC
    /// of the data buffer. Once `header.len() == HEADER_LENGTH`, this will have capacity equal to
    /// the data length plus `FOOTER_LENGTH`; before then, it will be an empty vector.
    data_plus_footer: Vec<u8>,
}

impl TfRecordState {
    pub fn new() -> Self {
        TfRecordState {
            header: Vec::with_capacity(HEADER_LENGTH),
            data_plus_footer: Vec::new(),
        }
    }
}

pub struct TfRecord {
    pub data: Vec<u8>,
    data_crc: MaskedCrc,
}

/// A buffer's checksum was computed, but it did not match the expected value.
#[derive(Debug)]
pub struct ChecksumError {
    /// The actual checksum of the buffer.
    got: MaskedCrc,
    /// The expected checksum.
    want: MaskedCrc,
}

impl TfRecord {
    /// Validates the integrity of the record by computing its CRC-32 and checking it against the
    /// expected value.
    pub fn checksum(&self) -> Result<(), ChecksumError> {
        let got = compute_crc(&self.data);
        let want = self.data_crc;
        if got == want {
            Ok(())
        } else {
            Err(ChecksumError { got, want })
        }
    }

    /// Writes this TFRecord back to serialized form. This includes the header and footer in
    /// addition to the raw payload.
    ///
    /// The data checksum is taken from the original stored value and is not re-computed from the
    /// data buffer. Thus, if the original record was corrupt, then the newly written record will
    /// be corrupt, too.
    #[allow(dead_code)]
    pub fn write<W: Write>(&self, w: &mut W) -> io::Result<()> {
        let length_field = (self.data.len() as u64).to_le_bytes();
        w.write_all(&length_field)?;
        w.write_all(&compute_crc(&length_field).0.to_le_bytes())?;
        w.write_all(&self.data)?;
        w.write_all(&self.data_crc.0.to_le_bytes())?;
        Ok(())
    }
}

#[derive(Debug)]
pub enum ReadRecordError {
    /// Length field failed checksum. Cannot read rest of file.
    BadLengthCrc(ChecksumError),
    /// No hard-errors so far, but the record is not complete. Call `read_record` again with the
    /// same state buffer once new data may have been written to the file.
    Truncated,
    /// Record is too large to be represented in memory on this system.
    TooLarge(u64),
    /// Underlying I/O error.
    Io(io::Error),
}

impl From<io::Error> for ReadRecordError {
    fn from(io: io::Error) -> Self {
        ReadRecordError::Io(io)
    }
}

/// Attempts to read a TFRecord, behaving nicely in the face of truncations. If the record is
/// truncated, the result is a `Truncated` error, and the state buffer will be updated to contain
/// the prefix of the raw record that was read. The same state buffer should be passed to a
/// subsequent call to `read_record` that it may continue where it left off.
///
/// The record's length field is always validated against its checksum, but the full data is only
/// validated if you call `checksum()` on the resulting record.
pub fn read_record<R: Read>(
    st: &mut TfRecordState,
    reader: &mut R,
) -> Result<TfRecord, ReadRecordError> {
    if st.header.len() < st.header.capacity() {
        read_remaining(reader, &mut st.header)?;

        let (length_buf, length_crc_buf) = st.header.split_at(LENGTH_CRC_OFFSET);
        let length_crc = MaskedCrc(LittleEndian::read_u32(length_crc_buf));
        let actual_crc = compute_crc(length_buf);
        if length_crc != actual_crc {
            return Err(ReadRecordError::BadLengthCrc(ChecksumError {
                got: actual_crc,
                want: length_crc,
            }));
        }

        let length = LittleEndian::read_u64(length_buf);
        let data_plus_footer_length_u64 = length + (FOOTER_LENGTH as u64);
        let data_plus_footer_length = data_plus_footer_length_u64 as usize;
        if data_plus_footer_length as u64 != data_plus_footer_length_u64 {
            return Err(ReadRecordError::TooLarge(length));
        }
        st.data_plus_footer.reserve_exact(data_plus_footer_length);
    }

    if st.data_plus_footer.len() < st.data_plus_footer.capacity() {
        read_remaining(reader, &mut st.data_plus_footer)?;
    }

    let data_length = st.data_plus_footer.len() - FOOTER_LENGTH;
    let data_crc_buf = st.data_plus_footer.split_off(data_length);
    let data = std::mem::take(&mut st.data_plus_footer);
    let data_crc = MaskedCrc(LittleEndian::read_u32(&data_crc_buf));
    st.header.clear(); // reset, though the caller shouldn't use this again
    Ok(TfRecord { data, data_crc })
}

fn read_remaining<R: Read>(reader: &mut R, buf: &mut Vec<u8>) -> Result<(), ReadRecordError> {
    let want = buf.capacity() - buf.len();
    reader.take(want as u64).read_to_end(buf)?;
    if buf.len() < buf.capacity() {
        return Err(ReadRecordError::Truncated);
    }
    Ok(())
}
